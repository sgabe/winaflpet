package main

import (
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestCreateDB(t *testing.T) {
	dataDir := viper.GetString("data.dir")
	dataSrc := filepath.Join(dataDir, DB_SOURCE)
	assert.True(t, fileExists(dataSrc))
}
