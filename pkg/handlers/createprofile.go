package handlers

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "regexp"
    "strings"
    "time"

    "github.com/arnnvv/peeple-api/db"
    "github.com/arnnvv/peeple-api/pkg/enums"
    "github.com/arnnvv/peeple-api/pkg/token"
)

func respondError(w http.ResponseWriter, msg string, code int) {
    w.WriteHeader(code)
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success": false,
        "message": msg,
    })
}

func CreateProfile(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    fmt.Println("\n=== Starting Profile Creation ===")
    defer fmt.Println("=== End Profile Creation ===")

    // Get UserID from token
    claims, ok := r.Context().Value(token.ClaimsContextKey).(*token.Claims)
    if !ok || claims == nil {
        respondError(w, "Invalid token", http.StatusUnauthorized)
        return
    }
    fmt.Printf("[Auth] UserID from token: %d\n", claims.UserID)

    // Read and log raw request body
    bodyBytes, _ := io.ReadAll(r.Body)
    fmt.Printf("[Request] Raw body: %s\n", string(bodyBytes))
    r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

    // Fetch existing user
    var existingUser db.UserModel
    if err := db.DB.First(&existingUser, claims.UserID).Error; err != nil {
        fmt.Printf("[Error] User lookup failed: %v\n", err)
        respondError(w, "User not found", http.StatusNotFound)
        return
    }
    fmt.Printf("[Existing User] Phone: %v\n", existingUser.PhoneNumber)

    // Decode into update structure
    var profileData struct {
        Name             *string                      `json:"name"`
        LastName         *string                      `json:"last_name"`
        DateOfBirth      db.CustomDate                `json:"date_of_birth"`
        Latitude         *float64                     `json:"latitude"`
        Longitude        *float64                     `json:"longitude"`
        Gender           *enums.Gender                `json:"gender"`
        DatingIntention  *enums.DatingIntention       `json:"dating_intention"`
        Height           *string                      `json:"height"`
        Hometown         *string                      `json:"hometown"`
        JobTitle         *string                      `json:"job_title"`
        Education        *string                      `json:"education"`
        ReligiousBeliefs *enums.Religion              `json:"religious_beliefs"`
        DrinkingHabit    *enums.DrinkingSmokingHabits `json:"drinking_habit"`
        SmokingHabit     *enums.DrinkingSmokingHabits `json:"smoking_habit"`
        Prompts          []db.Prompt                  `json:"prompts"`
    }

    if err := json.NewDecoder(r.Body).Decode(&profileData); err != nil {
        fmt.Printf("[Decode Error] %T: %+v\n", err, err)
        respondError(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
        return
    }

    // Update existing user fields
    existingUser.Name = profileData.Name
    existingUser.LastName = profileData.LastName
    existingUser.DateOfBirth = profileData.DateOfBirth
    existingUser.Latitude = profileData.Latitude
    existingUser.Longitude = profileData.Longitude
    existingUser.Gender = profileData.Gender
    existingUser.DatingIntention = profileData.DatingIntention
    existingUser.Height = profileData.Height
    existingUser.Hometown = profileData.Hometown
    existingUser.JobTitle = profileData.JobTitle
    existingUser.Education = profileData.Education
    existingUser.ReligiousBeliefs = profileData.ReligiousBeliefs
    existingUser.DrinkingHabit = profileData.DrinkingHabit
    existingUser.SmokingHabit = profileData.SmokingHabit
    existingUser.Prompts = profileData.Prompts

    // Validate
    if err := validateProfile(&existingUser); err != nil {
        fmt.Printf("[Validation Failed] %s\n", err)
        respondError(w, err.Error(), http.StatusBadRequest)
        return
    }

    // Database operations
    fmt.Println("[Database] Starting save operation...")
    result := db.DB.Save(&existingUser)
    
    fmt.Printf("[Database] RowsAffected: %d\n", result.RowsAffected)
    if result.Error != nil {
        fmt.Printf("[Database Error] Type: %T | Error: %+v\n", result.Error, result.Error)
        respondError(w, "Database error: "+result.Error.Error(), http.StatusInternalServerError)
        return
    }

    // Handle prompts
    fmt.Printf("[Prompts] Processing %d prompts\n", len(existingUser.Prompts))
if len(existingUser.Prompts) > 0 {
    // Delete existing prompts
    if err := db.DB.Where("user_id = ?", existingUser.ID).Delete(&db.Prompt{}).Error; err != nil {
        fmt.Printf("[Prompts Delete Error] %v\n", err)
        respondError(w, "Failed to update prompts", http.StatusInternalServerError)
        return
    }
    
    // Create new prompts without IDs
    var newPrompts []db.Prompt
    for _, p := range existingUser.Prompts {
        newPrompts = append(newPrompts, db.Prompt{
            UserID:   existingUser.ID,
            Category: p.Category,
            Question: p.Question,
            Answer:   p.Answer,
        })
    }
    
    if err := db.DB.Create(&newPrompts).Error; err != nil {
        fmt.Printf("[Prompts Create Error] %v\n", err)
        respondError(w, "Failed to save prompts", http.StatusInternalServerError)
        return
    }
}

    fmt.Println("[Success] Profile updated successfully")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success": true,
        "message": "Profile updated successfully",
    })
}

// Keep validateProfile function unchanged from previous version
// Keep validateProfile function same as before
func validateProfile(user *db.UserModel) error {
    fmt.Println("\n=== Starting Profile Validation ===")
    defer fmt.Println("=== End Profile Validation ===\n")

    // Name validation
    fmt.Printf("[Validation] Checking name: %v\n", user.Name)
    if user.Name == nil || len(*user.Name) == 0 || len(*user.Name) > 20 {
        return fmt.Errorf("name must be between 1 and 20 characters")
    }

    // LastName validation
    fmt.Printf("[Validation] Checking last name: %v\n", user.LastName)
    if user.LastName != nil && len(*user.LastName) > 20 {
        return fmt.Errorf("last name must not exceed 20 characters")
    }

    // Date of Birth validation
    fmt.Printf("[Validation] Checking DOB: %v\n", user.DateOfBirth)
    if user.DateOfBirth.IsZero() {
        return fmt.Errorf("date of birth is required")
    }
    age := time.Since(time.Time(user.DateOfBirth)).Hours() / 24 / 365
    fmt.Printf("[Validation] Calculated age: %.2f years\n", age)
    if age < 18 {
        return fmt.Errorf("must be at least 18 years old")
    }

    // Location validation
    fmt.Printf("[Validation] Checking location - Lat: %v, Long: %v\n", user.Latitude, user.Longitude)
    if user.Latitude == nil || user.Longitude == nil {
        return fmt.Errorf("latitude and longitude are required")
    }

    // Height validation
    fmt.Printf("[Validation] Checking height: %v\n", user.Height)
    if user.Height == nil {
        return fmt.Errorf("height is required")
    }
    heightRegex := regexp.MustCompile(`^[4-6]'([0-9]|1[0-1])"$`)
    if !heightRegex.MatchString(*user.Height) {
        return fmt.Errorf("invalid height format")
    }

    // Required enum validations
    fmt.Printf("[Validation] Checking required enums:\nGender: %v\nDating Intention: %v\nReligious Beliefs: %v\nDrinking: %v\nSmoking: %v\n",
        user.Gender, user.DatingIntention, user.ReligiousBeliefs, user.DrinkingHabit, user.SmokingHabit)

    if user.Gender == nil || *user.Gender == "" {
        return fmt.Errorf("gender is required")
    }
    if user.DatingIntention == nil || *user.DatingIntention == "" {
        return fmt.Errorf("dating intention is required")
    }
    if user.ReligiousBeliefs == nil || *user.ReligiousBeliefs == "" {
        return fmt.Errorf("religious beliefs is required")
    }
    if user.DrinkingHabit == nil || *user.DrinkingHabit == "" {
        return fmt.Errorf("drinking habit is required")
    }
    if user.SmokingHabit == nil || *user.SmokingHabit == "" {
        return fmt.Errorf("smoking habit is required")
    }

    // Optional fields length validation
    fmt.Printf("[Validation] Checking optional fields:\nHometown: %v\nJob Title: %v\nEducation: %v\n",
        user.Hometown, user.JobTitle, user.Education)

    if user.Hometown != nil && len(*user.Hometown) > 15 {
        return fmt.Errorf("hometown must not exceed 15 characters")
    }
    if user.JobTitle != nil && len(*user.JobTitle) > 15 {
        return fmt.Errorf("job title must not exceed 15 characters")
    }
    if user.Education != nil && len(*user.Education) > 15 {
        return fmt.Errorf("education must not exceed 15 characters")
    }

    // Validate prompts
    fmt.Printf("[Validation] Checking prompts. Count: %d\n", len(user.Prompts))
    if len(user.Prompts) == 0 {
        return fmt.Errorf("at least one prompt is required")
    }

    for i, p := range user.Prompts {
        fmt.Printf("[Validation] Checking prompt %d:\nCategory: %s\nQuestion: %s\nAnswer: %s\n",
            i+1, p.Category, p.Question, p.Answer)

        if p.Category == "" {
            return fmt.Errorf("prompt category is required")
        }
        if p.Question == "" {
            return fmt.Errorf("prompt question is required")
        }
        if strings.TrimSpace(p.Answer) == "" {
            return fmt.Errorf("prompt answer cannot be empty")
        }

        // Validate prompt belongs to category
        promptType := enums.PromptType(p.Question)
        if promptType.GetCategory() != p.Category {
            return fmt.Errorf("prompt %s does not belong to category %s", p.Question, p.Category)
        }
    }

    fmt.Println("[Validation] All checks passed successfully")
    return nil
}
