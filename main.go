package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"runtime"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"

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

func getOffers(model string) (offers []string) {
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

	return
}

//

type state struct {
	db gorm.DB
}

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

func (self *BikeOffersResponse) getDbOffers(model string) (offers []string) {
	log.Println(self.st.db)
	return
}

func (self *BikeOffersResponse) SetOffers() {
	self.Offers = self.getDbOffers(self.Model)
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

		resp.SetOffers()

		err = encoder.Encode(resp)
		if err != nil {
			log.Println(err)
			encoder.Encode(Error{err.Error()})
		}
	}
}

func startBBSrv() {
	db, err := gorm.Open("mysql", "bb:123456@tcp(dbserver:3306)/bikes?charset=utf8&parseTime=True&loc=Local")
	if err != nil {
		log.Fatal(err)
	}

	db.DB().SetMaxIdleConns(10)
	db.DB().SetMaxOpenConns(100)

	st := &state{db: db}

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
