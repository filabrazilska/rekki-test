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

// email adresses not really able to be validated by regexes
var emailRegexp *regexp.Regexp;
func validateRegex(email string) (ValidatorResponse, string) {
    matches := emailRegexp.FindStringSubmatch(email)
    if matches == nil {
        return ValidatorResponse{
            Valid : false,
            Reason : "Email not validated by regex",
        }, ""
    }
    return ValidatorResponse{
        Valid : true,
    }, matches[1]
}

func validateMX(domain string) (ValidatorResponse, []*net.MX) {
    mxs, err := net.LookupMX(domain)
    if err != nil {
        log.Print(err.Error())
        return ValidatorResponse{
            Valid : false,
            Reason : "MX Lookup failed",
        }, nil
    }
    if len(mxs) == 0 {
        return ValidatorResponse{
            Valid : false,
            Reason : "No MX records for the domain",
        }, nil
    }
    return ValidatorResponse{
        Valid : true,
    }, mxs
}

// TODO: we should somehow cache the result of this so as not to open connections every time
func validateSMTPConnection(servers []*net.MX) ValidatorResponse {
    errors := make([]string,0)
    for _,s := range(servers) {
        conn, err := net.Dial("tcp", net.JoinHostPort(s.Host, "smtp"))
        defer conn.Close()
        if err == nil {
            return ValidatorResponse{
                Valid : true,
            }
        }
    }
    return ValidatorResponse{
        Valid : false,
        Reason : strings.Join(errors, ","),
    }
}

func writeResponse(w http.ResponseWriter, response Response) {
    output, err := json.Marshal(response)
    if err != nil {
        http.Error(w, "Cannot format response: " + err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("content-type", "application/json")
    w.Write(output)
}

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

    regexValidation, domain := validateRegex(params.Email)
    response.Validators["regex"] = regexValidation
    if ! regexValidation.Valid {
        response.Valid = false
        writeResponse(w, response)
        return
    }

    mxValidation, servers := validateMX(domain)
    response.Validators["mx"] = mxValidation
    if ! mxValidation.Valid {
        response.Valid = false
        writeResponse(w, response)
        return
    }

    connectValidation := validateSMTPConnection(servers)
    response.Validators["smtp"] = connectValidation
    if ! connectValidation.Valid {
        response.Valid = false
        writeResponse(w, response)
        return
    }

    writeResponse(w, response)
}

func initializeLocalData() {
    var err error
    emailRegexp, err = regexp.CompilePOSIX(`[a-zA-Z-]@([a-zA-Z].*$)`) // just a '@' surrounded by something word-like
    if err != nil {
        log.Fatal("Cannot initialize emailRegexp", err)
    }
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
