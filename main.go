package main

import (
    "encoding/json"
    "io/ioutil"
    "log"
    "os"
    "regexp"
    "net"
    "net/http"
    "strconv"
    "strings"
)

type RequestParams struct {
    Email   string `json:"email"`
}

type ValidatorResponse struct {
    Valid       bool `json:"valid"`
    Reason      string `json:"reason,omitempty"`
}

type Response struct {
    Valid       bool `json:"valid"`
    Validators  map[string]ValidatorResponse `json:"validators"`
}

// TODO: can we somehow pass info between validators so that eg. in the SMTP connect one I don't have to do a lookup again?
// TODO: also the domain part of address doesn't have to be extracted twice but just in the regex test

// email adresses not really able to be validated by regexes
var emailRegexp *regexp.Regexp;
func validateRegex(email string) ValidatorResponse {
    if !emailRegexp.MatchString(email) {
        return ValidatorResponse{
            Valid : false,
            Reason : "Email not validated by regex",
        }
    }
    return ValidatorResponse{
        Valid : true,
    }
}

func validateMX(email string) ValidatorResponse {
    domain := strings.SplitAfter(email, "@")[1]
    mxs, err := net.LookupMX(domain)
    if err != nil {
        log.Print(err.Error())
        return ValidatorResponse{
            Valid : false,
            Reason : "MX Lookup failed",
        }
    }
    if len(mxs) == 0 {
        return ValidatorResponse{
            Valid : false,
            Reason : "No MX records for the domain",
        }
    }
    return ValidatorResponse{
        Valid : true,
    }
}

// TODO
func validateSMTPConnection(email string) ValidatorResponse {
    return ValidatorResponse{
        Valid : true,
    }
}

var allValidators map[string]func(string)ValidatorResponse
func validateHandler(w http.ResponseWriter, r *http.Request) {
    var params RequestParams
    body, err := ioutil.ReadAll(r.Body)
    defer r.Body.Close()
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    err = json.Unmarshal(body, &params)
    if err != nil {
        http.Error(w, "Cannot unmarshal JSON: " + err.Error(), http.StatusBadRequest)
        return
    }
    if params.Email == "" {
        http.Error(w, "Mandatory parameter 'email' missing", http.StatusBadRequest)
        return
    }

    var response Response = Response{
        Valid: true,
        Validators: make(map[string]ValidatorResponse),
    }

    for i,v := range(allValidators) {
        r := v(params.Email)
        response.Valid = response.Valid && r.Valid
        response.Validators[i] = r
    }

    output, err := json.Marshal(response)
    if err != nil {
        http.Error(w, "Cannot format response: " + err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("content-type", "application/json")
    w.Write(output)
}

func initializeLocalData() {
    var err error
    emailRegexp, err = regexp.CompilePOSIX(`[a-zA-Z-]@[a-zA-Z]`) // just a '@' surrounded by something word-like
    if err != nil {
        log.Fatal("Cannot initialize emailRegexp", err)
    }
    allValidators = make(map[string]func(string)ValidatorResponse)
    allValidators["regex"] = validateRegex
    allValidators["domain"] = validateMX
    allValidators["smtp"] = validateSMTPConnection
}

func main() {
    var port string = os.Getenv("PORT")
    if port == "" {
        log.Fatal("Empty PORT variable")
    }

    _, err := strconv.Atoi(port)
    if err != nil {
        log.Fatal("Wrong PORT value: %s\n")
    }

    initializeLocalData()

    http.HandleFunc("/email/validate", validateHandler)

    log.Fatal(http.ListenAndServe(":" + port, nil))
}
