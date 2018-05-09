package redicrypt

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/garyburd/redigo/redis"
	"github.com/pkg/errors"
	"golang.org/x/crypto/acme/autocert"
)

type RediCrypt struct {
	Addr string
	Conn redis.Conn
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

// Get reads a certificate data from redis.
func (rc *RediCrypt) Get(ctx context.Context, name string) ([]byte, error) {
	key := redisKeyForName(name)
	fmt.Println("redicrypt: getting cert for key " + key)

	data := ""
	done := make(chan error)

	go func() {
		var err error

		data, err = redis.String(rc.Conn.Do("GET", key))
		if err == redis.ErrNil {
			fmt.Println("redicrypt: Get encountered ErrNil")
			done <- autocert.ErrCacheMiss
		} else {
			fmt.Println("redicrypt: encountered redis error: " + err.Error())
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

	fmt.Println("redicrypt: fetched cert with key " + key)
	return certBytes, nil
}

// Put writes the certificate data to redis.
func (rc *RediCrypt) Put(ctx context.Context, name string, data []byte) error {
	key := redisKeyForName(name)
	fmt.Println("redicrypt: writing cert for key ", key)

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
			fmt.Println("redicrypt: encountered redis error: " + err.Error())
			return errors.Wrap(err, "Put failed to Do")
		}
	}

	fmt.Println("redicrypt: wrote cert to redis with key: " + key)
	return nil
}

// Delete removes the specified redis key.
func (rc *RediCrypt) Delete(ctx context.Context, name string) error {
	key := redisKeyForName(name)
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
			return errors.Wrap(err, "Delete failed to Do")
		}
	}

	return nil
}

func redisKeyForName(name string) string {
	return fmt.Sprintf("redicrypt/%s", name)
}
