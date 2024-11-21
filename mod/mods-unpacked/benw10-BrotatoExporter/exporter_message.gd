extends Node

func _ready():
	pass

# iota at home
const MESSAGE_TYPE_KEEP_ALIVE = 0
const MESSAGE_TYPE_TIME_SERIES_FULL = MESSAGE_TYPE_KEEP_ALIVE+1
const MESSAGE_TYPE_TIME_SERIES_DIFF = MESSAGE_TYPE_TIME_SERIES_FULL+1

const MESSAGE_REASON_NONE = 0
const MESSAGE_REASON_SHOP_ENTERED = MESSAGE_REASON_NONE + 1
const MESSAGE_REASON_STARTED_WAVE = MESSAGE_REASON_SHOP_ENTERED + 1
const MESSAGE_REASON_RUN_ENDED = MESSAGE_REASON_STARTED_WAVE + 1
const MESSAGE_REASON_POLL = MESSAGE_REASON_RUN_ENDED + 1
const MESSAGE_REASON_CONNECT = MESSAGE_REASON_POLL + 1

static func make_keep_alive_message()->Dictionary:
	return {
		"message_type": MESSAGE_TYPE_KEEP_ALIVE,
		"message_timestamp": _get_time_microseconds(),
		"message_reason": MESSAGE_REASON_POLL,
	}
# dict encoder should implement `encode_dict(dict: Dictionary)`
static func make_time_series_full_message(dict_encoder, msg_reason: int, diff_dict: Dictionary) -> Dictionary:
	return {
		"message_type": MESSAGE_TYPE_TIME_SERIES_FULL,
		"message_timestamp": _get_time_microseconds(),
		"message_reason": msg_reason,
		"message_content_buf": dict_encoder.encode_dict(diff_dict),
	}
# dict encoder should implement `encode_dict(dict: Dictionary)`
# constructor for timeseries diff message - sends only diff from the last timeseries object
static func make_time_series_diff_message(dict_encoder, msg_reason: int, diff_dict: Dictionary) -> Dictionary:
	return {
		"message_type": MESSAGE_TYPE_TIME_SERIES_DIFF,
		"message_timestamp": _get_time_microseconds(),
		"message_reason": msg_reason,
		"message_content_buf": dict_encoder.encode_dict(diff_dict),
	}

static func _get_time_microseconds()->int:
	var t = Time.get_unix_time_from_system()
	return int(t * 1000 * 1000)

# copy the message to the buffer
# format is -
# - message type (uint8)
# - message reason (uint8)
# - message timestamp (int64)
# - optional ExporterDictSerializer.encode_dict output (see for spec.)
static func write_to_buf(msg: Dictionary, buf: StreamPeer)->int:
	if not msg:
		return 1

	buf.put_8(msg["message_type"])
	buf.put_8(msg["message_reason"])
	buf.put_64(msg["message_timestamp"])
	
	if !msg.has("message_content_buf") || msg["message_content_buf"].size() < 1:
		return OK

	#print("Message Content - ", message_content_buf)
	var error: int = buf.put_data(msg["message_content_buf"])
	return error
