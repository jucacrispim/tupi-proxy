tupi-proxy is a plugin for Tupi.

Install
=======

To install tupi-proxy first clone the code:

```sh
$ git clone https://github.com/jucacrispim/tupi-proxy
```

And then build the code:

```sh
$ cd tupi-proxy
$ make build
```

This will create the binary: ``./build/proxy_plugin.so``.

Usage
=====

To use the plugin with tupi, in  your config file put:

```toml
...
ServePlugin = "/path/to/proxy_plugin.so"
ServePluginConf = {
    "host" = "http://some.where:8901"
}
...
```

To preserve the original host in the proxy request use:

```toml
...
ServePlugin = "/path/to/proxy_plugin.so"
ServePluginConf = {
    "host" = "http://some.where:8901"
	"preserveHost" = true
}
...
```
