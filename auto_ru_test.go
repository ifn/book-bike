package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListAutoRuVendors(t *testing.T) {
	vendors, err := listAutoRuVendors()
	assert.NoError(t, err)
	assert.Contains(t, vendors, "honda")
	assert.NotContains(t, vendors, "sale")

	for _, vendor := range vendors {
		fmt.Println(vendor)
	}
}
