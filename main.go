package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

const AllMatches = -1

const AutoRuVendorsUrl = "http://moto.auto.ru/motorcycle/"
const AvitoMoscowUrl = "https://www.avito.ru/moskva/mototsikly_i_mototehnika/"

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

func modelToAvitoQuery(model string) string {
	return AvitoMoscowUrl
}

var modelToQuery map[site](func(string) string) = map[site](func(string) string){
	AutoRu: modelToAutoRuQuery,
	Avito:  modelToAvitoQuery,
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

func fetchAvitoOffers(html string, out chan interface{}) error {
	return nil
}

var fetchOffers map[site](func(string, chan interface{}) error) = map[site](func(string, chan interface{}) error){
	AutoRu: fetchAutoRuOffers,
	Avito:  fetchAvitoOffers,
}

func NewClient() *http.Transport {
	timeout := time.Duration(time.Second * 5)
	dialTimeout := func(network, addr string) (net.Conn, error) {
		return net.DialTimeout(network, addr, timeout)
	}

	return &http.Transport{
		Dial: dialTimeout,
	}
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

		req, err := http.NewRequest("GET", siteQuery, nil)
		if err != nil {
			return
		}

		ua := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.10; rv:40.0) Gecko/20100101 Firefox/40.0"
		req.Header.Set("User-Agent", ua)

		t := NewClient()

		// TODO: missing timeouts
		resp, err := t.RoundTrip(req)
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

func (self *BikeOffersResponse) getOffers(model string) (offers []string, err error) {

	processMessage := func(ch <-chan interface{}, msg interface{}) <-chan interface{} {
		switch msg := msg.(type) {
		case error:
			err = msg
			return nil
		case string:
			offers = append(offers, msg)
			return ch
		}
		return ch
	}

	auto := getAutoRuOffers(model)
	avito := getAvitoOffers(model)

	for auto != nil || avito != nil {
		select {
		case msg, ok := <-auto:
			if !ok {
				auto = nil
				continue
			}
			auto = processMessage(auto, msg)
		case msg, ok := <-avito:
			if !ok {
				avito = nil
				continue
			}
			avito = processMessage(avito, msg)
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

	Error

	*BikeOffersRequest
	Offers []string `json:"offers"`
}

//

func (self *BikeOffersResponse) SetOffers() (err error) {
	query := self.Model

	var offers []string

	defer func() {
		self.Offers = offers
	}()

	model, err := queryToModel(query)
	if err != nil {
		return
	}

	offers, err = self.getOffers(model)
	return
}

func getBikeOffers(st *state) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		decoder := json.NewDecoder(r.Body)
		encoder := json.NewEncoder(w)

		resp := &BikeOffersResponse{st: st}

		var err error

		defer func() {
			if err != nil {
				log.Println(err)
				resp.Err = err.Error()
			}

			err := encoder.Encode(resp)
			if err != nil {
				log.Println(err)
			}
		}()

		err = decoder.Decode(resp)
		if err != nil {
			return
		}

		err = resp.SetOffers()
	}
}

func startBikeSearcher() {
	st := new(state)

	r := mux.NewRouter()
	r.HandleFunc("/getBikeOffers", getBikeOffers(st)).Methods("POST")
	http.Handle("/", r)

	log.Fatal(http.ListenAndServe(":"+os.Getenv("PORT"), nil))
}

func main() {
	startBikeSearcher()
}

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
}
