package main

import (
	"fmt"
)

func getAutoOffers(model string) <-chan string {
	links := make(chan string)
	go func() {
		links <- "http://moto.auto.ru/motorcycle/used/sale/1826802-02dbed.html"
		close(links)
	}()
	return links
}

func getAvitoOffers(model string) <-chan string {
	links := make(chan string)
	go func() {
		links <- "https://www.avito.ru/moskva/mototsikly_i_mototehnika/honda_cbr_1000_rr_fireblade_632522473"
		close(links)
	}()
	return links
}

func getOffers(model string) {
	auto := getAutoOffers(model)
	avito := getAvitoOffers(model)

	for auto != nil || avito != nil {
		select {
		case link, ok := <-auto:
			if !ok {
				auto = nil
				break
			}
			fmt.Println(link)
		case link, ok := <-avito:
			if !ok {
				avito = nil
				break
			}
			fmt.Println(link)
		}
	}
}

func main() {
	getOffers("VFR800")
}
