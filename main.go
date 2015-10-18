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

type site int

const (
	AutoRu site = iota
	Avito
)

var undefModel = errors.New("Undefined model")

func queryToModel(query string) (string, error) {
	query_ := strings.ToUpper(query)

	if model, ok := normalForms[query_]; ok {
		return model, nil
	}
	return "", undefModel
}

func modelToAutoRuQuery(model string) string {
	modelIds := map[string]string{
		"VFR800": "7889",
		"R6":     "9605",
	}

	modelPaths := map[string]string{
		"VFR800": "used/honda/vfr/",
		"R6":     "used/yamaha/yzf-r6/",
	}

	modelParam := "m[]=" + modelIds[model]

	return AutoRuVendorsUrl + modelPaths[model] + "?" + modelParam
}

var modelToQuery map[site](func(string) string) = map[site](func(string) string){
	AutoRu: modelToAutoRuQuery,
}

func fetchAutoRuOffers(html string, out chan interface{}) error {
	// '\n' is not matched by default, so it should work well without 'ungreedy' flag
	var offersRE = regexp.MustCompile(`(?U)href="(.+)".+class="offer-list"`)

	matchedOffers := offersRE.FindAllStringSubmatch(html, AllMatches)
	for _, matchedOffer := range matchedOffers {
		out <- matchedOffer[1]
	}

	return nil
}

var fetchOffers map[site](func(string, chan interface{}) error) = map[site](func(string, chan interface{}) error){
	AutoRu: fetchAutoRuOffers,
}

func getOffers(site_ site, model string) <-chan interface{} {
	links := make(chan interface{})

	go func() {
		var err error

		defer func() {
			if err != nil {
				links <- err
			}

			close(links)
		}()

		siteQuery := modelToQuery[site_](model)

		// TODO: missing timeouts
		resp, err := http.Get(siteQuery)
		if err != nil {
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			err = errors.New(fmt.Sprintf("request: %s, status: %d", siteQuery, resp.StatusCode))
			return
		}

		respBody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return
		}

		err = fetchOffers[site_](string(respBody), links)
		if err != nil {
			return
		}
	}()

	return links
}

func getAutoRuOffers(model string) <-chan interface{} {
	return getOffers(AutoRu, model)
}

func getAvitoOffers(model string) <-chan interface{} {
	return getOffers(Avito, model)
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
	query := self.Model

	model, err := queryToModel(query)
	if err != nil {
		return err
	}

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

		handleError := func(err error) {
			log.Println(err)
			encoder.Encode(Error{err.Error()})
		}

		resp := &BikeOffersResponse{st: st}

		err := decoder.Decode(resp)
		if err != nil {
			handleError(err)
			return
		}

		err = resp.SetOffers()
		if err != nil {
			handleError(err)
			return
		}

		err = encoder.Encode(resp)
		if err != nil {
			handleError(err)
			return
		}
	}
}

func startBikeSearcher() {
	st := new(state)

	r := mux.NewRouter()
	r.HandleFunc("/getBikeOffers", getBikeOffers(st)).Methods("POST")
	http.Handle("/", r)

	n := negroni.Classic()
	n.UseHandler(r)
	n.Run(":" + os.Getenv("PORT"))
}

func main() {
	startBikeSearcher()
}

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
}
