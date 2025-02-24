package token

import (
    "encoding/json"
    "net/http"
    "os"
    "sync"
    "unicode"

    "github.com/arnnvv/peeple-api/db"
    "github.com/golang-jwt/jwt/v5"
    "gorm.io/gorm"
)

type TokenResponse struct {
    Success bool   `json:"success"`
    Message string `json:"message"`
}

var (
    jwtSecret     []byte
    jwtSecretOnce sync.Once
)

func getSecret() []byte {
    jwtSecretOnce.Do(func() {
        jwtSecret = []byte(os.Getenv("JWT_SECRET"))
    })
    return jwtSecret
}

func GenerateTokenHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")

    // Validate HTTP method
    if r.Method != http.MethodGet {
        w.WriteHeader(http.StatusMethodNotAllowed)
        json.NewEncoder(w).Encode(TokenResponse{
            Success: false,
            Message: "Only GET method allowed",
        })
        return
    }

    // Get phone number from query params
    phone := r.URL.Query().Get("phone")
    if phone == "" {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(TokenResponse{
            Success: false,
            Message: "Phone number is required",
        })
        return
    }

    // Validate phone number format
    if len(phone) != 10 {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(TokenResponse{
            Success: false,
            Message: "Phone number must be exactly 10 digits",
        })
        return
    }

    for _, c := range phone {
        if !unicode.IsDigit(c) {
            w.WriteHeader(http.StatusBadRequest)
            json.NewEncoder(w).Encode(TokenResponse{
                Success: false,
                Message: "Phone number must contain only digits",
            })
            return
        }
    }

    // Find user by phone number to get UserID
    var user db.UserModel
    result := db.DB.Where("phone_number = ?", phone).First(&user)
    if result.Error != nil {
        if result.Error == gorm.ErrRecordNotFound {
            w.WriteHeader(http.StatusNotFound)
            json.NewEncoder(w).Encode(TokenResponse{
                Success: false,
                Message: "User not found",
            })
            return
        }
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(TokenResponse{
            Success: false,
            Message: "Database error",
        })
        return
    }

    // Create and sign token with only UserID
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, &Claims{
        UserID: user.ID,
    })

    tokenString, err := token.SignedString(getSecret())
    if err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(TokenResponse{
            Success: false,
            Message: "Token generation failed",
        })
        return
    }

    // Return successful response
    json.NewEncoder(w).Encode(TokenResponse{
        Success: true,
        Message: tokenString,
    })
}
