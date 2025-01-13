extends Node

var ExporterMessage = preload("res://mods-unpacked/benw10-BrotatoExporter/exporter_message.gd")
var ModExporter = preload("res://mods-unpacked/benw10-BrotatoExporter/mod_exporter.gd")
var ExporterDictSerializer = preload("res://mods-unpacked/benw10-BrotatoExporter/exporter_dict_serializer.gd")
var ModGamePoller = preload("res://mods-unpacked/benw10-BrotatoExporter/mod_game_poller.gd")

const LOG_INFO = "BrotatoExporter"

const MOD_ID = "benw10-BrotatoExporter"
const _USER_CONFIG_NAME = "connect-user"

var _mod_exporter
var _conn_ready: bool = false

var _dict_serializer

var _config_data: Dictionary

var _game_poller

func _ready():
	var _error: int = 0
	
	ModLoaderLog.info(OS.get_user_data_dir(), LOG_INFO)
	
	var _config_file = File.new()
	
	if not _config_file.file_exists("res://mods-unpacked/benw10-BrotatoExporter/connect-config.json"):
		var configs = ModLoaderConfig.get_configs(MOD_ID)
		_config_data = configs["default"].data
		ModLoaderLog.info("Using default config", LOG_INFO)
	else:
		_error = _config_file.open("res://mods-unpacked/benw10-BrotatoExporter/connect-config.json", File.READ)
		if _error != OK:
			ModLoaderLog.fatal("Failed to load the config file", LOG_INFO)
			return
		var content = _config_file.get_as_text()
		
		var res = JSON.parse(content)
		if res.error != OK:
			ModLoaderLog.fatal("Invalid config format - " + res.error_string, LOG_INFO)
			return
		ModLoaderLog.info("Loaded connect-config", LOG_INFO)
		_config_data = res.get_result()

	if !_config_data["enabled"]:
		return

	_dict_serializer = ExporterDictSerializer.new()
	
	_mod_exporter = ModExporter.new()
	_error = _mod_exporter.connect("error", self, "_on_mod_exporter_err")
	_error = _mod_exporter.connect("connected", self, "_on_mod_exporter_connect")
	_error = _mod_exporter.connect("disconnected", self, "_on_mod_exporter_disconnect")
	_error = _mod_exporter.connect("authenticated", self, "_on_mod_exporter_authenticated")

	_mod_exporter.auth_token = _config_data["server_connection"]["auth_token"]
	_connect_exporter()

	add_child(_mod_exporter)

	_game_poller = ModGamePoller.new()
	add_child(_game_poller)

	_error = _game_poller.connect("diff_ready", self, "_on_diff_ready")
	
	ModLoaderLog.info("Ready", LOG_INFO)
	
func _on_mod_exporter_connect():
	ModLoaderLog.info("Connected to mod exporter", LOG_INFO)
	_conn_ready = true
	
func _on_mod_exporter_disconnect():
	ModLoaderLog.info("Disconnected from mod exporter", LOG_INFO)
	_conn_ready = false
	_connect_exporter()

func _on_mod_exporter_err(err_str: String, err_code: int):
	ModLoaderLog.info("Got error from exporter - (%s) - code (%d)" % [err_str,err_code], LOG_INFO)
	_connect_exporter()

# send new full message on authentication. This is so the server can be restarted without needing to
# worry about
func _on_mod_exporter_authenticated():
	_dict_serializer.clear()
	
func _connect_exporter():
	_dict_serializer.clear()
	var conn_dict = _config_data["server_connection"]
	ModLoaderLog.info("Connecting to %s:%d - using HTTPS (%s) - verify host (%s)" % [conn_dict["host"],conn_dict["port"],String(conn_dict["https"]),String(conn_dict["verify_host"])], LOG_INFO)
	_mod_exporter.connect_to_host(conn_dict["host"], conn_dict["port"], conn_dict["https"], conn_dict["verify_host"])

func _on_diff_ready(event_reason: String, diff: Dictionary):
	if diff.size() < 1:
		return
	# ModLoaderLog.info(event_reason + ": Diff - " + JSON.print(diff), LOG_INFO)
	var msg: Dictionary
	match event_reason:
		ModGamePoller.EVENT_REASON_ENTERED_SHOP:
			msg = ExporterMessage.make_time_series_full_message(_dict_serializer, ExporterMessage.MESSAGE_REASON_SHOP_ENTERED, diff)
		ModGamePoller.EVENT_REASON_STARTED_WAVE:
			msg = ExporterMessage.make_time_series_full_message(_dict_serializer, ExporterMessage.MESSAGE_REASON_STARTED_WAVE, diff)
		ModGamePoller.EVENT_REASON_RUN_ENDED:
			msg = ExporterMessage.make_time_series_full_message(_dict_serializer, ExporterMessage.MESSAGE_REASON_RUN_ENDED, diff)
		ModGamePoller.EVENT_REASON_POLL:
			msg = ExporterMessage.make_time_series_diff_message(_dict_serializer, ExporterMessage.MESSAGE_REASON_POLL, diff)
		_:
			return

	_mod_exporter.enqueue_message(msg)
