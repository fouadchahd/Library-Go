package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"io/ioutil"
	"library/account"
	"library/book"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type Books []*book.Book

func getDns() string {
	var envs map[string]string
	envs, err := godotenv.Read(".env")

	if err != nil {
		log.Fatal("Error loading .env file")
	}

	username, ok1 := envs["SERVER_USERNAME"]
	pass, ok2 := envs["SERVER_PASS"]
	dbName, ok3 := envs["DB_NAME"]

	if !(ok1 && ok2 && ok3) {
		log.Printf(" %v %v %v", username, pass, dbName)
		log.Fatal("Missing Env variable to start")
		os.Exit(1)
	}
	var dns strings.Builder
	dns.WriteString(username)
	dns.WriteString(":" + pass)
	dns.WriteString("@tcp(localhost:8889)/")
	dns.WriteString(dbName)
	dns.WriteString("?charset=utf8mb4&parseTime=True&loc=Local")
	return dns.String()
}

var Dns = getDns()
var db *gorm.DB
var DATE_FORMAT = "2006-01-02"

func connectToDb(connectionString string) *gorm.DB {
	db, err := gorm.Open(mysql.Open(connectionString), &gorm.Config{})
	if err != nil {
		panic("can't connect to database")
	}
	return db
}

func init() {
	db = connectToDb(Dns)
}

func main() {
	db.AutoMigrate(&book.Book{})
	db.AutoMigrate(&book.Borrow{})
	db.AutoMigrate(&account.Account{})
	db.AutoMigrate(&account.Meta{})

	r := mux.NewRouter().StrictSlash(true)
	handleBookRequests(r)
	handleAccountRequests(r)
	log.Println("Listening On :8000 ...")
	log.Fatal(http.ListenAndServe(":8000", r))
}

// Routes
func handleAccountRequests(r *mux.Router) {
	r.HandleFunc("/accounts/seed", seenAccountTable)
	r.HandleFunc("/accounts", getAccountListHandler).Methods(http.MethodGet)
	r.HandleFunc("/accounts", createAccountHandler).Methods(http.MethodPost)
	r.HandleFunc("/accounts/authorization", checkAccountAuthorization).Methods(http.MethodPost)
}

func handleBookRequests(r *mux.Router) {
	r.HandleFunc("/books", GetBooksHandler).Methods(http.MethodGet)
	r.HandleFunc("/books/{id}", GetSingleBooksHandler).Methods(http.MethodGet)
	r.HandleFunc("/books", CreateBookHandler).Methods(http.MethodPost)
	r.HandleFunc("/books/{id}/borrow", BorrowBook).Methods(http.MethodPost)
	r.HandleFunc("/books/isbn-{isbn}", func(writer http.ResponseWriter, request *http.Request) {
		DeleteBookHandler(writer, request, "isbn")
	}).Methods(http.MethodDelete)
	r.HandleFunc("/books/{id}", func(writer http.ResponseWriter, request *http.Request) {
		DeleteBookHandler(writer, request, "id")
	}).Methods(http.MethodDelete)
	r.HandleFunc("/", homeHandler).Methods(http.MethodGet)
}

func BorrowBook(w http.ResponseWriter, request *http.Request) {
	params := mux.Vars(request)
	bookID, err := strconv.Atoi(params["id"])
	if err != nil {
		json.NewEncoder(w).Encode(NewApiResponseError(err.Error()))
		return
	}

	headers := request.Header
	key := headers.Get("Key")
	token := headers.Get("Token")
	if len(key) == 0 || len(token) == 0 {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(NewApiResponseError("Missing Headers"))
		return
	}
	authorized := checkAuthorization(key, token)
	if !authorized {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(NewApiResponseError("Unauthorized"))
		return
	}
	//TODO: check book availability before borrowing

	// Create borrow record
	userId, _ := strconv.Atoi(key)
	log.Println("borrowed by ", userId)

	borrowRecord := book.Borrow{
		TransactionId: RandomString(20),
		BorrowDate:    time.Now(),
		DueDate:       nil,
		ReturnDate:    nil,
		BorrowedBy:    userId,
		BookId:        bookID,
	}
	createRow := db.Create(&borrowRecord)
	if createRow.Error != nil {
		json.NewEncoder(w).Encode(NewApiResponseError(createRow.Error.Error()))
		return
	}
	if createRow.RowsAffected != 1 {
		json.NewEncoder(w).Encode(NewApiResponseError("Something went wrong please retry again"))
		return
	}
	json.NewEncoder(w).Encode(NewApiResponseSuccess(borrowRecord.ID))
	return
}

// Account Handlers
type UserResponse struct {
	ID       uint              `json:"id"`
	Username string            `json:"username"`
	Password string            `json:"password"`
	Meta     map[string]string `json:"meta"`
}

func seenAccountTable(w http.ResponseWriter, request *http.Request) {
	row1 := db.Create(&account.Account{
		Username: "fouadchahd",
		Password: "password",
		Meta: []account.Meta{
			{
				Key:   "name",
				Value: "Fouad ElHamri",
			},
			{
				Key:   "phone",
				Value: "0645947757",
			},
		},
	})
	row2 := db.Create(&account.Account{
		Username: "fatiElhamri",
		Password: "password",
		Meta: []account.Meta{
			{
				Key:   "name",
				Value: "Fatimazehra Elhamri",
			},
			{
				Key:   "phone",
				Value: "0670964242",
			},
		},
	})
	if row1.Error != nil || row2.Error != nil {
		json.NewEncoder(w).Encode(NewApiResponseError(row1.Error.Error()))
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(NewApiResponseError("Account Created ..."))
}

func checkAccountAuthorization(w http.ResponseWriter, request *http.Request) {
	log.Printf(" POST /accounts/authorization")
	reqBody, _ := ioutil.ReadAll(request.Body)
	var requestMap map[string]string
	err := json.Unmarshal(reqBody, &requestMap)
	if err != nil {
		json.NewEncoder(w).Encode(*NewApiResponseError("UnAuthorized"))
		return
	}

	key, ok1 := requestMap["Key"]
	token, ok2 := requestMap["Token"]

	defer func(acc_id string) {
		log.Printf("check authorization for account id : %v", acc_id)
	}(key)

	if !(ok1 && ok2) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(*NewApiResponseError("UnAuthorized"))
		return
	}
	// Get account by ID
	var account account.Account
	getUserByIdRow := db.Where("id= ? AND password = ?", key, token).First(&account)
	if getUserByIdRow.Error != nil || getUserByIdRow.RowsAffected == 0 {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(*NewApiResponseError("UnAuthorized"))
		return
	}

	log.Println("AUTHORIZED")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(*NewApiResponseSuccess(convertAccount(account)))
}

func getAccountListHandler(w http.ResponseWriter, request *http.Request) {
	var accounts []account.Account
	var accountsResponse []UserResponse
	row := db.Model(&account.Account{}).Preload("Meta").Find(&accounts)
	if row.Error != nil {
		json.NewEncoder(w).Encode(NewApiResponseError(row.Error.Error()))
	}
	for _, account := range accounts {
		acc := convertAccount(account)
		accountsResponse = append(accountsResponse, *acc)
	}
	json.NewEncoder(w).Encode(NewApiResponseSuccess(accountsResponse))
}

func createAccountHandler(w http.ResponseWriter, request *http.Request) {

	reqBody, _ := ioutil.ReadAll(request.Body)
	var accountMap map[string]any
	err := json.Unmarshal(reqBody, &accountMap)
	if err != nil {
		json.NewEncoder(w).Encode(*NewApiResponseError(err.Error()))
		return
	}

	var newAccount account.Account
	newAccount.Username = accountMap["username"].(string)
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(accountMap["password"].(string)), 14)
	if err != nil {
		json.NewEncoder(w).Encode(*NewApiResponseError("trouble hashing the password provided"))
		return
	}
	newAccount.Password = string(hashedBytes)
	if !newAccount.IsValid() {
		json.NewEncoder(w).Encode(*NewApiResponseError("Invalid Data Provided"))
		return
	}
	var meta []account.Meta
	val, ok := accountMap["meta"]
	if ok {
		for key, val := range val.(map[string]any) {
			meta = append(meta, account.Meta{
				Key:   key,
				Value: val.(string),
			})
		}
	}
	newAccount.Meta = meta
	row := db.Create(&newAccount)
	if row.Error != nil {
		json.NewEncoder(w).Encode(*NewApiResponseError(row.Error.Error()))
		return
	}
	json.NewEncoder(w).Encode(*NewApiResponseSuccess(convertAccount(newAccount)))
	return

}

func checkAuthorization(key string, token string) bool {
	body := fmt.Sprintf(`{"Key": "%v","Token":"%v"}`, key, token)
	jsonBody := []byte(body)
	bodyReader := bytes.NewReader(jsonBody)
	requestURL := fmt.Sprintf("http://localhost:%d/accounts/authorization", 8000)
	req, err := http.NewRequest(http.MethodPost, requestURL, bodyReader)
	if err != nil {
		return false
	}
	client := http.Client{
		Timeout: 30 * time.Second,
	}

	res, err := client.Do(req)
	if err != nil {
		fmt.Printf("client: error making http request: %s\n", err)
		return false
	}
	resBody, _ := ioutil.ReadAll(res.Body)
	var apiResponse book.ApiResponse
	_ = json.Unmarshal(resBody, &apiResponse)
	return apiResponse.Status == book.Success
}

func convertAccount(account account.Account) *UserResponse {
	acc := UserResponse{
		Meta: make(map[string]string),
	}
	acc.ID = account.ID
	acc.Username = account.Username
	acc.Password = account.Password
	for _, meta := range account.Meta {
		acc.Meta[meta.Key] = meta.Value
	}
	return &acc
}

// Book Handlers
func DeleteBookHandler(w http.ResponseWriter, request *http.Request, key string) {
	log.Printf("DELETE /books/:%v", key)
	params := mux.Vars(request)
	var res *book.ApiResponse
	if _, ok := params[key]; !ok {
		res = NewApiResponseError("no book id provided")
		w.WriteHeader(http.StatusBadRequest)
		jsonResponse, _ := json.Marshal(res)
		w.Write(jsonResponse)
		return
	}
	//REMOVE FROM DB
	row := db.Exec("DELETE FROM book WHERE book.id = ? OR book.isbn = ?", params[key], params[key])
	err := row.Error
	if err != nil || row.RowsAffected == 0 {
		json.NewEncoder(w).Encode(*NewApiResponseError(""))
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(*NewApiResponseSuccess("Procedure Went Successfully"))
}

func CreateBookHandler(w http.ResponseWriter, request *http.Request) {
	log.Printf("POST /books")
	reqBody, _ := ioutil.ReadAll(request.Body)
	var book book.Book
	err := json.Unmarshal(reqBody, &book)
	if valid, _ := book.IsValid(); !valid || &err != nil {
		json.NewEncoder(w).Encode(*NewApiResponseError("Invalid Data Provided"))
		return
	}
	if err != nil {
		json.NewEncoder(w).Encode(*NewApiResponseError("Something Went Wrong"))
		return
	}
	res := db.Create(&book)
	err = res.Error
	if err != nil {
		json.NewEncoder(w).Encode(*NewApiResponseError(err.Error()))
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(*NewApiResponseSuccess(book))
}

func GetSingleBooksHandler(w http.ResponseWriter, req *http.Request) {
	log.Printf("GET /books/:id")

	params := mux.Vars(req)
	var res *book.ApiResponse
	if _, ok := params["id"]; !ok {
		res = NewApiResponseError("no book id provided")
		w.WriteHeader(http.StatusBadRequest)
		jsonResponse, _ := json.Marshal(res)
		w.Write(jsonResponse)
		return
	}
	var book book.Book
	row := db.First(&book, params["id"])
	if row.Error != nil {
		json.NewEncoder(w).Encode(*NewApiResponseError(row.Error.Error()))
		return
	}
	res = NewApiResponseSuccess(book)
	jsonResponse, _ := json.Marshal(res)
	w.WriteHeader(http.StatusOK)
	w.Write(jsonResponse)
}

func GetBooksHandler(w http.ResponseWriter, _ *http.Request) {
	log.Println("GET /books")
	var books Books
	db.Find(&books)
	res := NewApiResponseSuccess(books)
	jsonVar, _ := json.Marshal(res)
	w.Write(jsonVar)
}

// Response Constructor
func NewApiResponseError(msg string) *book.ApiResponse {
	return &book.ApiResponse{
		Data:   msg,
		Status: book.Error,
	}
}

func NewApiResponseSuccess(data any) *book.ApiResponse {
	return &book.ApiResponse{
		Data:   data,
		Status: book.Success,
	}
}

func homeHandler(resp http.ResponseWriter, _ *http.Request) {
	_, err := fmt.Fprint(resp, "Welcome in our Library Services \n"+
		"\n##Book\n"+
		"-Get List of Books : /books\n"+
		"-Get Single Book : /books/:id\n"+
		"-Remove Book by id : /books/:id\n"+
		"-Create New Book : /books\n"+
		"\n##Account\n"+
		"-Seed Account : /accounts/seed\n"+
		"-Get List of Accounts : /accounts\n"+
		"-Get Single Account : /accounts/:id\n"+
		"-Remove Account by id : /accounts/:id\n"+
		"-Create New Account : /accounts\n")
	if err != nil {
		resp.WriteHeader(http.StatusInternalServerError)
		json, _ := json.Marshal("Internal Server Error")
		resp.Write(json)
	}
	resp.WriteHeader(http.StatusOK)
}

func RandomString(n int) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	s := make([]rune, n)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return string(s)
}
