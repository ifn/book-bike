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

func fetchAutoRuOffers(html string, out chan interface{}) error {
	// '\n' is not matched by default, so it should work well without 'ungreedy' flag
	var offersRE = regexp.MustCompile(`(?U)href="(.+)".+class="offer-list"`)

	matchedOffers := offersRE.FindAllStringSubmatch(html, AllMatches)
	for _, matchedOffer := range matchedOffers {
		out <- matchedOffer[1]
	}

	return nil
}

func getAutoRuOffers(model string) <-chan interface{} {
	links := make(chan interface{})

	go func() {
		defer close(links)

		autoRuQuery := modelToAutoRuQuery(model)

		resp, err := http.Get(autoRuQuery)
		if err != nil {
			links <- err
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			links <- errors.New(fmt.Sprintf("request: %s, status: %d", autoRuQuery, resp.StatusCode))
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
