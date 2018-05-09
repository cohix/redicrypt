# redicrypt

A LetsEncrypt cert cache for redis

redicrypt is a drop-in replacement for the default `autocert.DirCache` in the `acme` package.

Example:
```
m := &autocert.Manager{
	Cache:      redicrypt.RediCryptWithAddr("redis:6379"),
	Prompt:     autocert.AcceptTOS,
	HostPolicy: autocert.HostWhitelist(hosts...),
}
```

redicrypt is useful to circumvent LetsEncrypt rate limits when ephemeral containers requesting the same certs are in use. A persistent redis instance caching the cert files will allow containers to request the same certs across multiple invocations without needing to persist any of their filesystem.