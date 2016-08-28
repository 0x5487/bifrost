package main

import "github.com/jasonsoft/napnap"

func identity(c *napnap.Context, next napnap.HandlerFunc) {
	key := c.Request.Header.Get("Authorization")
	var consumer Consumer
	if len(key) == 0 {
		consumer = Consumer{}
		_logger.debug("no key")
		c.Set("consumer", consumer)
		next(c)
		return
	}

	token, err := _tokenRepo.Get(key)
	if err != nil {
		panic(err)
	}
	if token == nil {
		consumer = Consumer{}
		_logger.debug("key was not found")
		c.Set("consumer", consumer)
		next(c)
		return
	}

	if token.isValid() == false {
		err := _tokenRepo.Delete(token.ID)
		if err != nil {
			panic(err)
		}
		consumer = Consumer{}
		_logger.debug("key has expired")
		c.Set("consumer", consumer)
		next(c)
		return
	}

	// verify client's ip which must be the same as token's ip address.
	if _config.Token.VerifyIP {
		clientIP := getClientIP(c.RemoteIPAddress())
		_logger.debugf("consumer ip: %v", clientIP)
		if len(token.IPAddress) > 0 && token.IPAddress != clientIP {
			consumer = Consumer{}
			_logger.debug("token didn't match client ip")
			c.Set("consumer", consumer)
			next(c)
			return
		}
	}

	target, err := _consumerRepo.Get(token.ConsumerID)
	if err != nil {
		panic(err)
	}
	if target == nil {
		consumer = Consumer{}
		_logger.debug("consumer was not found")
		c.Set("consumer", consumer)
		next(c)
		return
	}

	// extend token's life
	if _config.Token.SlidingExpiration {
		token.renew()
		_tokenRepo.Update(token)
	}

	consumer = *(target)
	_logger.debugf("consumer id: %v", consumer.ID)
	c.Set("consumer", consumer)
	next(c)
}
