class_name ExporterDictSerializer
extends Node

# type headers for binary data
# used to tell the server what data types we are working with
# TODO: add types with more depth
# TODO: improve space usage by making strings and ints use correct amount of bytes to represent the number

# represents any utf-8 encoded portion of data
# immediately following this a uint32 indicating the length of the string should
# be provided
const SERIAL_TYPE_STRING = 0xdb

const SERIAL_TYPE_INT8 = 0xd0
const SERIAL_TYPE_INT16 = 0xd1
const SERIAL_TYPE_INT32 = 0xd2
const SERIAL_TYPE_INT64 = 0xd3

const SERIAL_TYPE_FLOAT32 = 0xca

const MESSAGE_DICT_MAPPING_HEADER = 0xdf

# stores dictionary keys' (strings) mappings to a [$keyMapping: PoolByteArray, $data_type: SERIAL_TYPE] pair
var _dict_key_mapping_dict = Dictionary()

var _key_counter: int = 0

# by using 2 bytes per key, we can have a max of 65,535 distinct keys
var _key_counter_max: int = 1 << 31

static func serial_type_for_int(val: int)->int:
	if -(1 << 7) <= val and val <= (1 << 7):
		return SERIAL_TYPE_INT8
	if -(1 << 15) <= val and val <= (1 << 15):
		return SERIAL_TYPE_INT16
	if -(1 << 31) <= val and val <= (1 << 31):
		return SERIAL_TYPE_INT32
	
	return SERIAL_TYPE_INT64

# need to clear this on new session start
func clear():
	_key_counter = 0
	_dict_key_mapping_dict = Dictionary()

# returns [error: String|null, isNew: bool, $keyMapping: uint16, $data_type: SERIAL_TYPE]
func _get_mapping_for_key(key: String, value):
	if _dict_key_mapping_dict.has(key):
		var v = _dict_key_mapping_dict.get(key)
		return [null, false, v[0], v[1]]
		
	if _key_counter >= _key_counter_max:
		return ["Key count overflow - too many distinct dict keys"]
	
	var serial_type: int
	match typeof(value):
		TYPE_INT:
			serial_type = serial_type_for_int(value)
		TYPE_REAL:
			serial_type = SERIAL_TYPE_FLOAT32
		TYPE_STRING:
			serial_type = SERIAL_TYPE_STRING
		_:
			return ["Unknown type given"]
	
	var item = [_key_counter, serial_type]
	_dict_key_mapping_dict[key] = item.duplicate()
	
	_key_counter = _key_counter + 1
	
	return [null, true, item[0], item[1]]

# encodes any given dictionary - looks something like:
# - MESSAGE_DICT_MAPPING_HEADER (uint8)
# - new key count (uint16)
# - (for each new key)
# -- key mapping (uint16)
# -- SERIAL_TYPE (uint8)
# -- key string length (uint16)
# -- key string (variable [uint8, ...])
# - amount of key-value pairs (uint16)
# - (for each key in dict)
# -- $key_mappings[key] (uint16)
# -- value bytes (variable - either uintx or [uint32 {length}, uint8, uint8, ...]
# TODO: make this more efficient by pre-allocating the arrays
func encode_dict(dict: Dictionary) -> PoolByteArray:
	var header_buf = StreamPeerBuffer.new()
	# header + dict byte count + 
	header_buf.put_data(PoolByteArray([MESSAGE_DICT_MAPPING_HEADER, 0, 0]))
	
	var dict_buf = StreamPeerBuffer.new()
	dict_buf.put_data(PoolByteArray([0, 0]))
	
	var new_key_count = 0
	# can't just use dict len in-case parsing on keys fails
	var total_key_count = 0
	for key in dict:
		# sanity check
		if key.length() < 1:
			continue
		
		var val = dict[key]
		var res = _get_mapping_for_key(key, val)
		
		if res[0]:
			var err_msg = "Got error for key ("+key+") - "+res[0]
			print("Error - ", err_msg)
			continue
		
		var is_new = res[1]
		var key_mapping = res[2]
		var serial_type = res[3]
		
		# check to see if the type changed (hits for larger ints)
		var val_serial_type: int
		match typeof(val):
			TYPE_INT:
				val_serial_type = serial_type_for_int(val)
			TYPE_STRING:
				val_serial_type = SERIAL_TYPE_STRING
			TYPE_REAL:
				val_serial_type = SERIAL_TYPE_FLOAT32
			_:
				pass
		if not val_serial_type:
			continue
		
		if is_new:
			var error = _write_key_mapping_to_header(header_buf, key, key_mapping, serial_type)
			if error != OK:
				print("Got error code for mapping header - ", error)
				continue
			
			new_key_count = new_key_count + 1
		elif val_serial_type != serial_type:
			serial_type = val_serial_type
			
			_dict_key_mapping_dict[key] = [key_mapping, serial_type]
			var error = _write_key_mapping_to_header(header_buf, key, key_mapping, serial_type)
			if error != OK:
				print("Got error code for mapping header - ", error)
				continue
			
			new_key_count = new_key_count + 1
		
		var error = _write_key_value_pair(dict_buf, key_mapping, val, serial_type)
		if error != OK:
			print("Got error code for kv - ", error)
			continue
		
		total_key_count = total_key_count + 1
		
	var header_res = header_buf.data_array
	var dict_res = dict_buf.data_array
		
	# encode counts in little endian
	if new_key_count > 0:
		var new_key_length_bytes = int16_bytes(new_key_count)
		# first byte is MESSAGE_DICT_MAPPING_HEADER
		header_res.set(1, new_key_length_bytes[0])
		header_res.set(2, new_key_length_bytes[1])
	
	var dict_len_bytes = int16_bytes(total_key_count)
	dict_res.set(0, dict_len_bytes[0])
	dict_res.set(1, dict_len_bytes[1])
	
	header_res.append_array(dict_res)
	
	return header_res

# appends below to given buf
# - key mapping (uint16)
# - ExporterDictMapping.SERIAL_TYPE (uint8)
# - key string length (uint16)
# - key string (variable [uint8, ...])
static func _write_key_mapping_to_header(buf: StreamPeerBuffer, key: String, key_mapping: int, serial_type: int)->int:
	buf.put_16(key_mapping)
	buf.put_8(serial_type)
	# length of key string
	buf.put_16(key.length())
	# actual key string in utf8
	var error = buf.put_data(key.to_utf8())
	
	return error

# appends below to given buf
# -- $key_mappings[key] (uint16)
# -- value bytes (variable - either uint32 or [uint32 {length}, uint8, uint8, ...]
static func _write_key_value_pair(buf: StreamPeerBuffer, key_mapping: int, value, value_serial_type: int)->int:
	var error: int = 0
	buf.put_16(key_mapping)
	
	match value_serial_type:
		SERIAL_TYPE_INT8:
			buf.put_8(value)
		SERIAL_TYPE_INT16:
			buf.put_16(value)
		SERIAL_TYPE_INT32:
			buf.put_32(value)
		SERIAL_TYPE_INT64:
			buf.put_64(value)
		SERIAL_TYPE_FLOAT32:
			buf.put_float(value)
		SERIAL_TYPE_STRING:
			# for strings - its int32 for length then string content
			buf.put_32(value.length())
			error = buf.put_data(value.to_utf8())
	
	return error

static func int16_bytes(v: int)->Array:
	var b1 = v & 0xFF
	var b2 = (v >> 8) & 0xFF
	var b3 = (v >> 16) & 0xFF
	var b4 = (v >> 24) & 0xFF
	
	return [b1, b2, b3, b4]
