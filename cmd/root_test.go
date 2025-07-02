package cmd

import (
	"testing"

	"github.com/katiem0/gh-bbc-exporter/internal/data"
	"github.com/katiem0/gh-bbc-exporter/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestValidateExportFlagsMixedAuth(t *testing.T) {
	// Test case for mixed authentication methods
	cmdFlags := &data.CmdFlags{
		BitbucketToken:   "testtoken",
		BitbucketUser:    "testuser",
		BitbucketAppPass: "testpass",
	}

	err := utils.ValidateExportFlags(cmdFlags)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mixed authentication methods")
}
