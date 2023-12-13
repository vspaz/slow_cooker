package http_client

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestOneHeaderPairOk(t *testing.T) {
	headerToValue := GetHeaders("key1: value1")
	assert.Equal(t, "value1", headerToValue["key1"])
}

func TestMultipleHeaderPairOk(t *testing.T) {
	headerToValue := GetHeaders("key1: value1, key2: value2, key3: value3")
	assert.Equal(t, "value1", headerToValue["key1"])
	assert.Equal(t, "value2", headerToValue["key2"])
	assert.Equal(t, "value3", headerToValue["key3"])
}

func TestBadlyFormattedHeaderStringOk(t *testing.T) {
	badlyFormattedHeaderString := " key1:value1,    key2: value2,key3:  value3 , "
	headerToValue := GetHeaders(badlyFormattedHeaderString)
	assert.Equal(t, 3, len(headerToValue))
	assert.Equal(t, "value1", headerToValue["key1"])
	assert.Equal(t, "value2", headerToValue["key2"])
	assert.Equal(t, "value3", headerToValue["key3"])
}

func TestMissingHeaderNameOk(t *testing.T) {
	headerToValue := GetHeaders(" :value1")
	assert.Empty(t, headerToValue)
}

func TestNoHeadersOk(t *testing.T) {
	assert.Empty(t, GetHeaders(" "))
}
