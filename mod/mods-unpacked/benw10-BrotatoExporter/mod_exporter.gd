class_name ModExporter
extends Node

var ExporterMessage = preload("res://mods-unpacked/benw10-BrotatoExporter/exporter_message.gd")

signal authenticated()
signal error(err, code)
signal connected()
signal disconnected()

# public fields and methods
# must set before attempting to do anything
var auth_token: String

func conn_ready() -> bool:
	return _conn_ready

# should implement `write_to_buf(buf: StreamPeerBuffer)`
func enqueue_message(msg):
	# circle back around if we overflow 1000 messages - will overwrite the oldest messages first
	if _message_queue.size() >= _MAX_MESSAGES && _message_queue_idx >= _MAX_MESSAGES-1:
		_message_queue_idx = 0
	elif _message_queue.size() <= _message_queue_idx:
		_message_queue.append_array(_empty_array.duplicate())

	_message_queue[_message_queue_idx] = msg
	_message_queue_idx = _message_queue_idx + 1
	
static func status_str(v: int) -> String:
	match v:
		HTTPClient.STATUS_DISCONNECTED:
			return "STATUS_DISCONNECTED"
		HTTPClient.STATUS_RESOLVING:
			return "STATUS_RESOLVING"
		HTTPClient.STATUS_CANT_RESOLVE:
			return "STATUS_CANT_RESOLVE"
		HTTPClient.STATUS_CONNECTING:
			return "STATUS_CONNECTING"
		HTTPClient.STATUS_CANT_CONNECT:
			return "STATUS_CANT_CONNECT"
		HTTPClient.STATUS_CONNECTED:
			return "STATUS_CONNECTED"
		HTTPClient.STATUS_REQUESTING:
			return "STATUS_REQUESTING"
		HTTPClient.STATUS_BODY:
			return "STATUS_BODY"
		HTTPClient.STATUS_CONNECTION_ERROR:
			return "STATUS_CONNECTION_ERROR"
		HTTPClient.STATUS_SSL_HANDSHAKE_ERROR:
			return "STATUS_SSL_HANDSHAKE_ERROR"
	return ""
	
func connect_to_host(host: String, port: int, use_https: bool = false, verify_host: bool = false) -> void:
	_authenticated = false
	# Reset status so we can tell if it changes to error again.
	_status = _client.STATUS_DISCONNECTED
	var error: int = _client.connect_to_host(host, port, use_https, verify_host)
	if error != OK:
		emit_signal("error", "Error connecting to host", error)
		return
	
# private fields and methods
# 4 seconds so we dont timeout
const _MAX_POLL_FREQ = 4000
var _poll_freq: int = 1000

const _MAX_MESSAGES: int = 1000

var _status: int = 0

var _client: HTTPClient = HTTPClient.new()

var _last_frame_processed_ticks: int = 0

var _body_buf: StreamPeerBuffer = StreamPeerBuffer.new()

func _ready() -> void:
	_status = _client.get_status()
	_last_frame_processed_ticks = Time.get_ticks_msec() - _poll_freq
	_body_buf.resize(1024)
	
	var _error_exited_tree = self.connect("tree_exited", self, "_on_tree_exited")

var _empty_array: Array = [null, null, null, null, null, null, null, null, null, null]

# array of ExporterMessages
var _message_queue: Array = _empty_array.duplicate()
var _message_queue_idx: int = 0

var _conn_ready: bool = false
var _authenticated: bool = false

var _session_token: String
var _session_token_exp_ticks: int

const _AUTH_REQ: int = 1
const _POST_MESSAGE_REQ: int = _AUTH_REQ + 1

const _AUTH_ENDPOINT: String = "/api/auth/authenticate"
const _POST_MESSAGE_ENDPOINT: String = "/api/message/post"

var _in_req: int = 0

const CONNECTION_ERROR = 5

func _process(_delta: float) -> void:
	var _error: int = _client.poll()

	var new_status: int = _client.get_status()
	if new_status != _status:
		_status = new_status
		match _status:
			_client.STATUS_DISCONNECTED:
				emit_signal("disconnected")
				_conn_ready = false
			_client.STATUS_CONNECTED:
				if !_conn_ready:
					emit_signal("connected")
				_conn_ready = true
			_client.STATUS_CONNECTION_ERROR, _client.STATUS_CANT_RESOLVE, _client.STATUS_CANT_CONNECT, _client.STATUS_SSL_HANDSHAKE_ERROR:
				_conn_ready = false
				emit_signal("error", "Socket in error state - " + status_str(_status), CONNECTION_ERROR)
	
	if !_conn_ready:
		return
				
	var in_req_res = _handle_in_request()
	if !in_req_res:
		return

	var now_ticks = Time.get_ticks_msec()

	# check or send every second
	# allows messages to queue up or authentication calls to be throttled
	if now_ticks - _last_frame_processed_ticks < _poll_freq:
		return

	if !_authenticated:
		_error = _start_authenticate()
	else:
		enqueue_message(ExporterMessage.make_keep_alive_message())
		_error = _start_send_queue()
	# do backoff on error
	if _error != OK:
		_poll_freq = _poll_freq * 2
		if _poll_freq > _MAX_POLL_FREQ:
			_poll_freq = _MAX_POLL_FREQ
	
	_last_frame_processed_ticks = now_ticks
	
# returns whether to continue or not
# handles behavior while doing a request
# returns [err: Error, should_continue: bool]
func _handle_in_request() -> bool:
	var _error: int = 0
	if _status == _client.STATUS_BODY:
		var chunk = _client.read_response_body_chunk()
		_error = _body_buf.put_data(chunk)
		return false

	if _status != _client.STATUS_CONNECTED:
		return false

	if _in_req == _POST_MESSAGE_REQ:
		_in_req = 0
		
		var resp_code = _client.get_response_code()
		if resp_code > 400 && resp_code < 410:
			_authenticated = false
		
		if resp_code != 200:
			emit_signal("error", "Queue POST returned non 200 response - " + _body_buf.data_array.get_string_from_utf8(), resp_code)
			_poll_freq = _poll_freq * 2
			if _poll_freq > _MAX_POLL_FREQ:
				_poll_freq = _MAX_POLL_FREQ
				
			_body_buf.clear()
			
			if resp_code == 500:
				_message_queue = _empty_array.duplicate()
				_message_queue_idx = 0
		
			return false
		
		_body_buf.clear()
		# clear queue only after req succeeded
		_message_queue = _empty_array.duplicate()
		_message_queue_idx = 0
		
		# on success clear backoff
		_poll_freq = 1000

	if _in_req == _AUTH_REQ:
		_in_req = 0
		
		var resp_code = _client.get_response_code()
		if resp_code != 200:
			emit_signal("error", "Auth GET returned non 200 response - " + String(resp_code), 1)
			_body_buf.clear()
			_poll_freq = _poll_freq * 2
			if _poll_freq > _MAX_POLL_FREQ:
				_poll_freq = _MAX_POLL_FREQ

			return false
		
		_body_buf.big_endian = false
		_body_buf.seek(0) # since we appended a bunch of data we will be at the end
		# microsecond UTC Unix Epoch
		var session_token_exp: int = _body_buf.get_64()
		# calculate microseconds until
		var now_seconds: float = Time.get_unix_time_from_system()
		# convert to milliseconds for ms ticks
		_session_token_exp_ticks = int(session_token_exp/1000) - int(now_seconds * 1000)
		var token_len: int = _body_buf.get_16()
		
		var session_token_res = _body_buf.get_data(token_len)
		if session_token_res[0] != OK:
			emit_signal("error", "Failed to get token - " + String(resp_code), 1)
			_body_buf.clear()
			_poll_freq = _poll_freq * 2
			if _poll_freq > _MAX_POLL_FREQ:
				_poll_freq = _MAX_POLL_FREQ
	
			return false
		_session_token = PoolByteArray(session_token_res[1]).get_string_from_utf8()
		if not _session_token || _session_token.length() < 1:
			emit_signal("error", "Failed to get parse token - " + String(resp_code), 1)
			return false
			
		_authenticated = true

		_body_buf.clear()
		
		# on success clear backoff
		_poll_freq = 1000
		# once we authenticate emit signal so that callers know a new session started
		emit_signal("authenticated")
	return true
	
func _start_authenticate() -> int:
	if not auth_token || _in_req != 0:
		return 1
	
	var headers = [
		"Content-Type: application/octet-stream",
		"Authorization: Bearer " + auth_token
	]
	
	var error: int = _client.request(HTTPClient.METHOD_POST, _AUTH_ENDPOINT, headers)
	if error != OK:
		emit_signal("error", "Error making request - " + status_str(_status), error)
		return 1

	_in_req = _AUTH_REQ

	return 0

func _start_send_queue() -> int:
	if !_authenticated || _in_req != 0:
		return 0

	var _buf: StreamPeerBuffer = StreamPeerBuffer.new()

	for msg in _message_queue:
		if not msg:
			continue
		var error: int = ExporterMessage.write_to_buf(msg, _buf)
		if error != OK:
			emit_signal("error", "Error writing to stream", error)
			return 1
			
	if _buf.data_array.size() < 1:
		return 0

	var headers = [
		"Content-Type: application/octet-stream",
		"Authorization: JWT " + _session_token
	]

	var error: int = _client.request_raw(HTTPClient.METHOD_POST, _POST_MESSAGE_ENDPOINT, headers, _buf.data_array)
	if error != OK:
		emit_signal("error", "Error making request - " + status_str(_status), error)
		return 1

	_in_req = _POST_MESSAGE_REQ
	
	return 0

func _on_tree_exiting():
	_client.close()
