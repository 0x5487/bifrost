package main

import "github.com/jasonsoft/napnap"

func identity(c *napnap.Context, next napnap.HandlerFunc) {
	key := c.Request.Header.Get("Authorization")
	var consumer Consumer

	if len(key) == 0 {
		consumer = Consumer{}
		_logger.debug("no key")
	} else {
		token := _tokenRepo.Get(key)
		if token == nil {
			consumer = Consumer{}
			_logger.debug("key was not found")
		}

		if token.isValid() == false {
			err := _tokenRepo.Delete(token.Key)
			if err != nil {
				panic(err)
			}
			consumer = Consumer{}
			_logger.debug("key was invalid")
		}

		target := _consumerRepo.Get(token.ConsumerID)
		if target == nil {
			consumer = Consumer{}
			_logger.debug("consumer was not found")
		} else {
			consumer = *(target)
			_logger.debugf("consumer id: %v", consumer.ID)
		}
	}

	c.Set("_consumer", consumer)
	next(c)
}
