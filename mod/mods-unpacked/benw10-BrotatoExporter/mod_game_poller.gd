class_name ModGamePoller
extends Node

const EVENT_REASON_ENTERED_SHOP = "entered_shop"
const EVENT_REASON_STARTED_WAVE = "started_wave"
const EVENT_REASON_RUN_ENDED = "run_ended"
const EVENT_REASON_POLL = "poll"

signal _scene_changed(scene)
signal diff_ready(event_reason, diff)

const _default_scene_poll_dur = 0.02

# started on shop enter and stopped on entered main menu or ended run
var _game_poll_timer: Timer

const _default_game_poll_dur = 2

func _ready():
	var _error: int = 0
	_game_poll_timer = Timer.new()
	_game_poll_timer.wait_time = _default_game_poll_dur
	_game_poll_timer.one_shot = false
	add_child(_game_poll_timer)
	
	_error = _game_poll_timer.connect("timeout", self, "_on_game_timer_timeout")
	
	var scene_timer = Timer.new()
	scene_timer.wait_time = _default_scene_poll_dur
	scene_timer.one_shot = false
	add_child(scene_timer)
	
	_error = scene_timer.connect("timeout", self, "_on_scene_timer_timeout")
	scene_timer.start()
	
	_error = self.connect("_scene_changed", self, "_on_scene_changed")

var cur_scene: Node
var last_scene: Node

func _on_scene_timer_timeout():
	cur_scene = get_tree().current_scene
	if cur_scene == last_scene:
		return
	
	emit_signal("_scene_changed", cur_scene)
	
	last_scene = cur_scene
	
var _in_run = false
var _in_wave = false

func _on_scene_changed(scene: Node):
	var reason_str: String
	if scene is BaseShop:
		reason_str = EVENT_REASON_ENTERED_SHOP
		if _game_poll_timer.is_stopped():
			_game_poll_timer.start()
		_in_run = true
		_in_wave = false
	if scene is TitleScreen:
		_game_poll_timer.stop()
		_in_run = false
		_in_wave = false
		# on title screen open notify that the 
		_last_stats = Dictionary({ "current_character": "-" })
		_last_effects = Dictionary()
		emit_signal("diff_ready", EVENT_REASON_POLL, _last_stats)
	if scene is Main:
		reason_str = EVENT_REASON_STARTED_WAVE
		# handle start of the game case
		if _game_poll_timer.is_stopped():
			_game_poll_timer.start()
		_in_wave = true
		_in_run = true
	if scene is BaseEndRun:
		reason_str = EVENT_REASON_RUN_ENDED
		_game_poll_timer.stop()
		_in_wave = false
	
	if not _in_run:
		return
	
	emit_signal("diff_ready", reason_str, full_stat_dict(0))

func full_stat_dict(player_index: int) -> Dictionary:
	_last_stats = Dictionary()
	_last_effects = Dictionary()
	
	return _get_stat_diff(player_index)

func _on_game_timer_timeout():
	var diff_dict = _get_stat_diff(0)

	emit_signal("diff_ready", EVENT_REASON_POLL, diff_dict)

var _last_stats: Dictionary
var _last_effects: Dictionary
var _cur_stats: Dictionary
var _cur_effects: Dictionary

# returns dict that is flattened - all children are in the top-level obj
func _get_stat_diff(player_index: int)->Dictionary:
	var diff = Dictionary()
	_cur_stats = RunData.players_data[player_index].serialize().duplicate(false)

	for key in _cur_stats:
		# special case for effects for now. This includes the actual stats like luck but is flattened to the top for simplicity
		if key == "effects":
			# stats obj not deep duplicated
			_cur_effects = _cur_stats[key].duplicate(false)
			
			for effects_key in _cur_effects:
				_put_diff(effects_key, diff, _last_effects, _cur_effects, "effects_")
			
			_last_effects = _cur_effects
			
			continue
		
		_put_diff(key, diff, _last_stats, _cur_stats)
	
	_last_stats = _cur_stats

	return diff

func _put_diff(key: String, diff: Dictionary, last_dict: Dictionary, cur_dict: Dictionary, diff_prefix: String = ""):
	var last
	if last_dict:
		last = last_dict.get(key, null)
	var cur = cur_dict[key]
	# don't recurse into the random stuff for now - most of the useful stuff
	# is at this level anyways
	match typeof(cur):
		TYPE_STRING, TYPE_INT, TYPE_REAL:
			pass
		_:
			return

	if last != cur:
		diff[diff_prefix+key] = cur
