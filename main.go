package main

import (
	"database/sql"
	_ "database/sql"
	"encoding/json"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	validation "github.com/go-ozzo/ozzo-validation/v3"
	"github.com/go-ozzo/ozzo-validation/v3/is"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"referrals/mylog"
	"strings"
	"time"
)

type ErrorCode struct {
	Code         string `json:"code"`
	InternalCode string `json:"internalCode"`
	DevMessage   string `json:"devMessage"`
	UserMessage  UserMessage
}
type UserMessage struct {
	LangRu string `json:"lang_ru"`
	LangEu string `json:"lang_eu"`
}

type ReferralRequest struct {
	ReferralPhone string  `json:"referralPhone"`
	ReferralEmail string  `json:"referralEmail"`
}
type LinkHash struct {
	Link string `json:"link"`
}

func (rr *ReferralRequest) Validate() error  {
	return validation.ValidateStruct(
		rr,
		validation.Field(&rr.ReferralEmail, validation.By(RequiredIf(rr.ReferralPhone == "")), is.Email),
		validation.Field(&rr.ReferralPhone, validation.By(RequiredIf(rr.ReferralEmail == ""))),
	)
}

func RequiredIf(cond bool) validation.RuleFunc  {
	return func(value interface{}) error {
		if cond {
			return validation.Validate(value, validation.Required)
		}
		return nil
	}
}


func (link *LinkHash) GenerateLinkHash() {
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))

	b := make([]byte, 10)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	link.Link = string(b)
}

func main() {
	r := mux.NewRouter()
	r.NotFoundHandler = http.HandlerFunc(NotFountHandler)
	r.MethodNotAllowedHandler = http.HandlerFunc(NotFountHandler)

	r.HandleFunc("/getReferralLink", ReferralLinkHandler).Methods("POST")
	r.HandleFunc("/L/{hash}", HashHandler).Methods("GET")
	file, _ := os.Create("referral.log")
	defer file.Close()
	http.ListenAndServe(":81", mylog.Handler(r,file))
}


func ReferralLinkHandler(w http.ResponseWriter, r *http.Request)  {
	authorization := r.Header.Get("authorization")
	jwtToken := strings.ReplaceAll(authorization, "Bearer " , "")

	profileId, err := jwtTokenReadAndValid(jwtToken)
	if err != nil {
		BadRequestHandler(w, err)
		return
	}


	var rr ReferralRequest
	if err = json.NewDecoder(r.Body).Decode(&rr);err != nil {
		BadRequestHandler(w,err)
		return
	}
	err = rr.Validate()
	if err != nil {
		BadRequestHandler(w, err)
		return
	}
	db, err := sql.Open("postgres", "postgres://postgres:postgres@localhost/postgres?sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err = db.Ping(); err != nil {
		log.Fatal(err)
	}
	var LinkHash LinkHash
	LinkHash.GenerateLinkHash()


	db.QueryRow("INSERT INTO referral (referrer_phone, referrer_email,profile_id,link_hash) VALUES ($1, $2, $3, $4)", NewNullString(rr.ReferralPhone), NewNullString(rr.ReferralEmail), profileId, LinkHash.Link)
	w.Header().Set("Content-Type", "application/json")
	if err = json.NewEncoder(w).Encode(LinkHash);err != nil {
		log.Print(err)
	}
}

func HashHandler(w http.ResponseWriter, r *http.Request)  {
	v := mux.Vars(r)

	db, err := sql.Open("postgres", "postgres://postgres:postgres@localhost/postgres?sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err = db.Ping(); err != nil {
		log.Fatal(err)
	}
	db.QueryRow("UPDATE referral SET open_at = $1 WHERE link_hash = $2", time.Now(), v["hash"])
	http.Redirect(w, r, "https://getapp.o-plati.by/?utm_source=referral", 301)
	return
}

func NewNullString(s string) sql.NullString {
	if len(s) == 0 {
		return sql.NullString{}
	}
	return sql.NullString{
		String: s,
		Valid: true,
	}
}

func jwtTokenReadAndValid(inToken interface{}) (interface{}, error) {
	publicKey, err := ioutil.ReadFile("oplati-public.pem")

	publicRSA, err := jwt.ParseRSAPublicKeyFromPEM(publicKey)
	if err != nil {
		return nil, err
	}
	token, err := jwt.Parse(inToken.(string), func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return publicRSA, err
	})

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		fmt.Println(claims["sub"], claims["exp"])
		return claims["sub"], nil
	} else {
		return "", nil
	}

}



func NotFountHandler(w http.ResponseWriter, r *http.Request) {
	notFound := ErrorCode{
		Code:         "404",
		InternalCode: "404",
		DevMessage:   "Не найден",
		UserMessage: UserMessage{
			LangEu: "Not Found",
			LangRu: "Не найден",
		},
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	err := json.NewEncoder(w).Encode(notFound)
	if err != nil {
		log.Fatal(err)
	}
}

func BadRequestHandler(w http.ResponseWriter, err error)  {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	validationError := ErrorCode{
		Code:         "400",
		InternalCode: "400",
		DevMessage:   err.Error(),
		UserMessage: UserMessage{
			LangEu: "Error",
			LangRu: "Ошибка",
		},
	}
	e := json.NewEncoder(w).Encode(validationError)
	if e != nil {
		log.Fatal(e)
	}
}