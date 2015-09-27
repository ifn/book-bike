package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"strings"

	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"
)

const AllMatches = -1

const AutoRuVendorsUrl = "http://moto.auto.ru/motorcycle/"

func queryToModel(query string) string {
	normal_forms := map[string]string{
		"VFR800": "VFR800", "ВЫФЕР": "VFR800",
		"R6": "R6", "YZF-R6": "R6", "СТРЕКОЗА": "R6",
	}
	return normal_forms[strings.ToUpper(query)]
}

func queryToAutoRuQuery(query string) string {
	model := queryToModel(query)

	model_ids := map[string]string{
		"VFR800": "7889",
		"R6": "9605",
	}

	model_param := "m[]=" + model_ids[model]

	model_paths := map[string]string{
		"VFR800": "used/honda/vfr/",
		"R6":     "used/yamaha/yzf-r6/",
	}

	return AutoRuVendorsUrl + model_paths[model] + "?" + model_param
}

func fetchAutoRuOffers(html string, out chan interface{}) error {
	// TODO: check re
	var offersRE = regexp.MustCompile(`(?U)href="(.+)".+class="offer-list"`)

	matchedOffers := offersRE.FindAllStringSubmatch(html, AllMatches)
	for _, matchedOffer := range matchedOffers {
		out <- matchedOffer[1]
	}

	return nil
}

func getAutoRuOffers(query string) <-chan interface{} {
	links := make(chan interface{})

	go func() {
		// TODO: check defer order
		defer close(links)

		auto_ru_query := queryToAutoRuQuery(query)

		resp, err := http.Get(auto_ru_query)
		if err != nil {
			links <- err
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			links <- errors.New(fmt.Sprintf("request: %s, status: %d", auto_ru_query, resp.StatusCode))
			return
		}

		respBody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			links <- err
			return
		}

		err = fetchAutoRuOffers(string(respBody), links)
		if err != nil {
			links <- err
			return
		}
	}()

	return links
}

func getAvitoOffers(query string) <-chan interface{} {
	links := make(chan interface{})
	go func() {
		links <- "https://www.avito.ru/moskva/mototsikly_i_mototehnika/honda_cbr_1000_rr_fireblade_632522473"
		close(links)
	}()
	return links
}

func (self *BikeOffersResponse) getOffers(query string) (offers []string, err error) {
	auto := getAutoRuOffers(query)
	var avito chan interface{}

Loop:
	for auto != nil || avito != nil {
		select {
		case msg, ok := <-auto:
			if !ok {
				auto = nil
				continue Loop
			}
			switch msg := msg.(type) {
			case error:
				err = msg
				auto = nil
				continue Loop
			case string:
				offers = append(offers, msg)
			}
		}
	}

	return
}

//

type state struct{}

//

type Error struct {
	Err string `json:"error"`
}

type BikeOffersRequest struct {
	Model string `json:"model"`
}

type BikeOffersResponse struct {
	st *state

	*BikeOffersRequest
	Offers []string `json:"offers"`
}

//

func (self *BikeOffersResponse) SetOffers() error {
	model := self.Model

	offers, err := self.getOffers(model)
	if err != nil {
		return err
	}

	self.Offers = offers
	return nil
}

func getBikeOffers(st *state) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		decoder := json.NewDecoder(r.Body)
		encoder := json.NewEncoder(w)

		resp := &BikeOffersResponse{st: st}

		err := decoder.Decode(resp)
		if err != nil {
			log.Println(err)
			encoder.Encode(Error{err.Error()})
			return
		}

		err = resp.SetOffers()
		if err != nil {
			log.Println(err)
			encoder.Encode(Error{err.Error()})
			return
		}

		err = encoder.Encode(resp)
		if err != nil {
			log.Println(err)
			encoder.Encode(Error{err.Error()})
			return
		}
	}
}

func startBBSrv() {
	st := new(state)

	r := mux.NewRouter()
	r.HandleFunc("/getBikeOffers", getBikeOffers(st)).Methods("POST")
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
