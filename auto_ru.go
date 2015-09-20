package main

import (
	"io/ioutil"
	"net/http"
	"regexp"
)

const AllMatches = -1

const AutoRuVendorsUrl = "http://moto.auto.ru/motorcycle/"

func listAutoRuVendors() (vendors []string, err error) {
	var vendorsRE = regexp.MustCompile(`motorcycle/used/(\w+)/`)

	notVendors := map[string]struct{}{
		"sale": struct{}{},
	}

	resp, err := http.Get(AutoRuVendorsUrl)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	matchedVendors := vendorsRE.FindAllStringSubmatch(string(respBody), AllMatches)
	for _, matchedVendor := range matchedVendors {
		vendor := matchedVendor[1]
		if _, ok := notVendors[vendor]; !ok {
			vendors = append(vendors, vendor)
		}
	}

	return
}
