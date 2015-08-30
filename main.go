package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"runtime"

	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"
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

type Error struct {
	Err string `json:"error"`
}

type BikeOffersRequest struct {
	Model string `json:"model"`
}

type BikeOffersResponse struct {
	*BikeOffersRequest
	Offers []string `json:"offers"`
}

func (self *BikeOffersResponse) SetOffers() {
	model := self.Model
	var offers []string

	auto := getAutoOffers(model)
	avito := getAvitoOffers(model)

	for auto != nil || avito != nil {
		select {
		case link, ok := <-auto:
			if !ok {
				auto = nil
				break
			}
			offers = append(offers, link)
		case link, ok := <-avito:
			if !ok {
				avito = nil
				break
			}
			offers = append(offers, link)
		}
	}

	self.Offers = offers
}

func getBikeOffers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	decoder := json.NewDecoder(r.Body)
	encoder := json.NewEncoder(w)

	resp := new(BikeOffersResponse)

	err := decoder.Decode(resp)
	if err != nil {
		log.Println(err)
		encoder.Encode(Error{err.Error()})
		return
	}

	resp.SetOffers()

	err = encoder.Encode(resp)
	if err != nil {
		log.Println(err)
		encoder.Encode(Error{err.Error()})
	}
}

func startBBSrv() {
	r := mux.NewRouter()
	r.HandleFunc("/getBikeOffers", getBikeOffers).Methods("POST")
	http.Handle("/", r)

	n := negroni.Classic()
	n.UseHandler(r)
	n.Run(":" + os.Getenv("PORT"))
}

func main() {
	startBBSrv()
}

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
}
