#!/usr/bin/env python3
"""Local ABI smoke test, synthetic temp state only; never installs into CPA."""
import base64
import ctypes as c
import json
import os
import sys
import tempfile

class Buffer(c.Structure):
    _fields_ = [("ptr", c.c_void_p), ("length", c.c_size_t)]
class Host(c.Structure):
    _fields_ = [("version", c.c_uint32), ("context", c.c_void_p), ("call", c.c_void_p), ("free", c.c_void_p)]
class Plugin(c.Structure):
    _fields_ = [("version", c.c_uint32), ("call", c.c_void_p), ("free", c.c_void_p), ("shutdown", c.c_void_p)]
lib = c.CDLL(sys.argv[1])
lib.cliproxy_plugin_init.argtypes = [c.POINTER(Host), c.POINTER(Plugin)]
lib.cliproxy_plugin_init.restype = c.c_int
host, plugin = Host(1, None, None, None), Plugin()
assert lib.cliproxy_plugin_init(c.byref(host), c.byref(plugin)) == 0
assert plugin.version == 1
call = c.CFUNCTYPE(c.c_int, c.c_char_p, c.c_void_p, c.c_size_t, c.POINTER(Buffer))(plugin.call)
free = c.CFUNCTYPE(None, c.c_void_p, c.c_size_t)(plugin.free)
shutdown = c.CFUNCTYPE(None)(plugin.shutdown)
def invoke(method, payload):
    raw = json.dumps(payload).encode()
    response = Buffer()
    result = call(method.encode(), raw, len(raw), c.byref(response))
    try:
        value = json.loads(c.string_at(response.ptr, response.length))
    finally:
        free(response.ptr, response.length)
    assert result == 0, value
    assert value['ok'], value
    return value['result']
with tempfile.TemporaryDirectory(prefix='device-native-test-') as directory:
    cfg = f'listen: 127.0.0.1:0\ndata_dir: {json.dumps(directory)}\n'
    request = {'schema_version': 10, 'config_yaml': base64.b64encode(cfg.encode()).decode()}
    assert invoke('plugin.register', request)['schema_version'] == 10
    assert invoke('management.register', {})['resources'][0]['Path'] == '/open'
    assert invoke('management.handle', {'Method': 'GET', 'Path': '/open'})['StatusCode'] == 200
    invoke('plugin.reconfigure', request)
    shutdown()
    shutdown()
    invoke('plugin.register', request)
    shutdown()
    assert os.stat(directory + '/admin-token').st_mode & 0o777 == 0o600
print('PASS: C ABI init/register/resource/reconfigure/shutdown/restart; private temp storage')
