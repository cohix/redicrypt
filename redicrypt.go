package redicrypt

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"

	"github.com/gomodule/redigo/redis"
	"github.com/pkg/errors"
	"golang.org/x/crypto/acme/autocert"
)

type RediCrypt struct {
	Addr   string
	Conn   redis.Conn
	Logger io.Writer
}

func RediCryptWithAddr(addr string) (*RediCrypt, error) {
	c, err := redis.Dial("tcp", addr)
	if err != nil {
		return nil, errors.Wrap(err, "RediCryptWithAddr failed to Dial")
	}

	rc := &RediCrypt{
		Addr: addr,
		Conn: c,
	}

	return rc, nil
}

// Get reads certificate data from redis.
func (rc *RediCrypt) Get(ctx context.Context, name string) ([]byte, error) {
	key := redisKeyForName(name)
	rc.log("getting cert for key " + key)

	data := ""
	done := make(chan error)

	go func() {
		var err error

		data, err = redis.String(rc.Conn.Do("GET", key))
		if err == redis.ErrNil {
			done <- autocert.ErrCacheMiss
		} else {
			done <- err
		}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-done:
		if err != nil {
			return nil, err
		}
	}

	certBytes, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, errors.Wrap(err, "Get failed to DecodeString")
	}

	return certBytes, nil
}

// Put writes certificate data to redis.
func (rc *RediCrypt) Put(ctx context.Context, name string, data []byte) error {
	key := redisKeyForName(name)
	rc.log("writing cert for key " + key)

	encodedData := base64.StdEncoding.EncodeToString(data)
	done := make(chan error)

	go func() {
		select {
		case <-ctx.Done():
			// Don't overwrite the file if the context was canceled.
		default:
			_, err := rc.Conn.Do("SET", key, encodedData)
			done <- err
		}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		if err != nil {
			return err
		}
	}

	return nil
}

// Delete removes the specified redis key.
func (rc *RediCrypt) Delete(ctx context.Context, name string) error {
	key := redisKeyForName(name)
	rc.log("removing cert for key " + key)

	done := make(chan error)

	go func() {
		_, err := rc.Conn.Do("DELETE", key)
		done <- err
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		if err != nil {
			return err
		}
	}

	return nil
}

// log writes to the io.Writer defined by Logger or standard out if Logger is not set.
// All input values are prefixed with "redicrypt: ".
func (rc *RediCrypt) log(input string) {
	if input == "" {
		return
	}
	output := fmt.Sprintf("redicrypt: %s", input)
	if rc == nil || rc.Logger == nil {
		fmt.Println(output)
		return
	}
	rc.Logger.Write([]byte(output))
}

func redisKeyForName(name string) string {
	return fmt.Sprintf("redicrypt/%s", name)
}
