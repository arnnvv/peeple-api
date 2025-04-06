// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.28.0

package migrations

import (
	"database/sql/driver"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

type AudioPrompt string

const (
	AudioPromptCanWeTalkAbout                  AudioPrompt = "canWeTalkAbout"
	AudioPromptCaptionThisPhoto                AudioPrompt = "captionThisPhoto"
	AudioPromptCaughtInTheAct                  AudioPrompt = "caughtInTheAct"
	AudioPromptChangeMyMindAbout               AudioPrompt = "changeMyMindAbout"
	AudioPromptChooseOurFirstDate              AudioPrompt = "chooseOurFirstDate"
	AudioPromptCommentIfYouveBeenHere          AudioPrompt = "commentIfYouveBeenHere"
	AudioPromptCookWithMe                      AudioPrompt = "cookWithMe"
	AudioPromptDatingMeIsLike                  AudioPrompt = "datingMeIsLike"
	AudioPromptDatingMeWillLookLike            AudioPrompt = "datingMeWillLookLike"
	AudioPromptDoYouAgreeOrDisagreeThat        AudioPrompt = "doYouAgreeOrDisagreeThat"
	AudioPromptDontHateMeIfI                   AudioPrompt = "dontHateMeIfI"
	AudioPromptDontJudgeMe                     AudioPrompt = "dontJudgeMe"
	AudioPromptMondaysAmIRight                 AudioPrompt = "mondaysAmIRight"
	AudioPromptABoundaryOfMineIs               AudioPrompt = "aBoundaryOfMineIs"
	AudioPromptADailyEssential                 AudioPrompt = "aDailyEssential"
	AudioPromptADreamHomeMustInclude           AudioPrompt = "aDreamHomeMustInclude"
	AudioPromptAFavouriteMemoryOfMine          AudioPrompt = "aFavouriteMemoryOfMine"
	AudioPromptAFriendsReviewOfMe              AudioPrompt = "aFriendsReviewOfMe"
	AudioPromptALifeGoalOfMine                 AudioPrompt = "aLifeGoalOfMine"
	AudioPromptAQuickRantAbout                 AudioPrompt = "aQuickRantAbout"
	AudioPromptARandomFactILoveIs              AudioPrompt = "aRandomFactILoveIs"
	AudioPromptASpecialTalentOfMine            AudioPrompt = "aSpecialTalentOfMine"
	AudioPromptAThoughtIRecentlyHadInTheShower AudioPrompt = "aThoughtIRecentlyHadInTheShower"
	AudioPromptAllIAskIsThatYou                AudioPrompt = "allIAskIsThatYou"
	AudioPromptGuessWhereThisPhotoWasTaken     AudioPrompt = "guessWhereThisPhotoWasTaken"
	AudioPromptHelpMeIdentifyThisPhotoBomber   AudioPrompt = "helpMeIdentifyThisPhotoBomber"
	AudioPromptHiFromMeAndMyPet                AudioPrompt = "hiFromMeAndMyPet"
	AudioPromptHowIFightTheSundayScaries       AudioPrompt = "howIFightTheSundayScaries"
	AudioPromptHowHistoryWillRememberMe        AudioPrompt = "howHistoryWillRememberMe"
	AudioPromptHowMyFriendsSeeMe               AudioPrompt = "howMyFriendsSeeMe"
	AudioPromptHowToPronounceMyName            AudioPrompt = "howToPronounceMyName"
	AudioPromptIBeatMyBluesBy                  AudioPrompt = "iBeatMyBluesBy"
	AudioPromptIBetYouCant                     AudioPrompt = "iBetYouCant"
	AudioPromptICanTeachYouHowTo               AudioPrompt = "iCanTeachYouHowTo"
	AudioPromptIFeelFamousWhen                 AudioPrompt = "iFeelFamousWhen"
	AudioPromptIFeelMostSupportedWhen          AudioPrompt = "iFeelMostSupportedWhen"
)

func (e *AudioPrompt) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = AudioPrompt(s)
	case string:
		*e = AudioPrompt(s)
	default:
		return fmt.Errorf("unsupported scan type for AudioPrompt: %T", src)
	}
	return nil
}

type NullAudioPrompt struct {
	AudioPrompt AudioPrompt
	Valid       bool // Valid is true if AudioPrompt is not NULL
}

// Scan implements the Scanner interface.
func (ns *NullAudioPrompt) Scan(value interface{}) error {
	if value == nil {
		ns.AudioPrompt, ns.Valid = "", false
		return nil
	}
	ns.Valid = true
	return ns.AudioPrompt.Scan(value)
}

// Value implements the driver Valuer interface.
func (ns NullAudioPrompt) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return string(ns.AudioPrompt), nil
}

type DateVibesPromptType string

const (
	DateVibesPromptTypeTogetherWeCould       DateVibesPromptType = "togetherWeCould"
	DateVibesPromptTypeFirstRoundIsOnMeIf    DateVibesPromptType = "firstRoundIsOnMeIf"
	DateVibesPromptTypeWhatIOrderForTheTable DateVibesPromptType = "whatIOrderForTheTable"
	DateVibesPromptTypeBestSpotInTown        DateVibesPromptType = "bestSpotInTown"
	DateVibesPromptTypeBestWayToAskMeOut     DateVibesPromptType = "bestWayToAskMeOut"
)

func (e *DateVibesPromptType) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = DateVibesPromptType(s)
	case string:
		*e = DateVibesPromptType(s)
	default:
		return fmt.Errorf("unsupported scan type for DateVibesPromptType: %T", src)
	}
	return nil
}

type NullDateVibesPromptType struct {
	DateVibesPromptType DateVibesPromptType
	Valid               bool // Valid is true if DateVibesPromptType is not NULL
}

// Scan implements the Scanner interface.
func (ns *NullDateVibesPromptType) Scan(value interface{}) error {
	if value == nil {
		ns.DateVibesPromptType, ns.Valid = "", false
		return nil
	}
	ns.Valid = true
	return ns.DateVibesPromptType.Scan(value)
}

// Value implements the driver Valuer interface.
func (ns NullDateVibesPromptType) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return string(ns.DateVibesPromptType), nil
}

type DatingIntention string

const (
	DatingIntentionLifePartner       DatingIntention = "lifePartner"
	DatingIntentionLongTerm          DatingIntention = "longTerm"
	DatingIntentionLongTermOpenShort DatingIntention = "longTermOpenShort"
	DatingIntentionShortTermOpenLong DatingIntention = "shortTermOpenLong"
	DatingIntentionShortTerm         DatingIntention = "shortTerm"
	DatingIntentionFiguringOut       DatingIntention = "figuringOut"
)

func (e *DatingIntention) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = DatingIntention(s)
	case string:
		*e = DatingIntention(s)
	default:
		return fmt.Errorf("unsupported scan type for DatingIntention: %T", src)
	}
	return nil
}

type NullDatingIntention struct {
	DatingIntention DatingIntention
	Valid           bool // Valid is true if DatingIntention is not NULL
}

// Scan implements the Scanner interface.
func (ns *NullDatingIntention) Scan(value interface{}) error {
	if value == nil {
		ns.DatingIntention, ns.Valid = "", false
		return nil
	}
	ns.Valid = true
	return ns.DatingIntention.Scan(value)
}

// Value implements the driver Valuer interface.
func (ns NullDatingIntention) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return string(ns.DatingIntention), nil
}

type DrinkingSmokingHabits string

const (
	DrinkingSmokingHabitsYes       DrinkingSmokingHabits = "yes"
	DrinkingSmokingHabitsSometimes DrinkingSmokingHabits = "sometimes"
	DrinkingSmokingHabitsNo        DrinkingSmokingHabits = "no"
)

func (e *DrinkingSmokingHabits) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = DrinkingSmokingHabits(s)
	case string:
		*e = DrinkingSmokingHabits(s)
	default:
		return fmt.Errorf("unsupported scan type for DrinkingSmokingHabits: %T", src)
	}
	return nil
}

type NullDrinkingSmokingHabits struct {
	DrinkingSmokingHabits DrinkingSmokingHabits
	Valid                 bool // Valid is true if DrinkingSmokingHabits is not NULL
}

// Scan implements the Scanner interface.
func (ns *NullDrinkingSmokingHabits) Scan(value interface{}) error {
	if value == nil {
		ns.DrinkingSmokingHabits, ns.Valid = "", false
		return nil
	}
	ns.Valid = true
	return ns.DrinkingSmokingHabits.Scan(value)
}

// Value implements the driver Valuer interface.
func (ns NullDrinkingSmokingHabits) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return string(ns.DrinkingSmokingHabits), nil
}

// Enumerated type for representing gender identity and/or sexual orientation as specified.
type GenderEnum string

const (
	GenderEnumMan      GenderEnum = "man"
	GenderEnumWoman    GenderEnum = "woman"
	GenderEnumGay      GenderEnum = "gay"
	GenderEnumLesbian  GenderEnum = "lesbian"
	GenderEnumBisexual GenderEnum = "bisexual"
)

func (e *GenderEnum) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = GenderEnum(s)
	case string:
		*e = GenderEnum(s)
	default:
		return fmt.Errorf("unsupported scan type for GenderEnum: %T", src)
	}
	return nil
}

type NullGenderEnum struct {
	GenderEnum GenderEnum
	Valid      bool // Valid is true if GenderEnum is not NULL
}

// Scan implements the Scanner interface.
func (ns *NullGenderEnum) Scan(value interface{}) error {
	if value == nil {
		ns.GenderEnum, ns.Valid = "", false
		return nil
	}
	ns.Valid = true
	return ns.GenderEnum.Scan(value)
}

// Value implements the driver Valuer interface.
func (ns NullGenderEnum) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return string(ns.GenderEnum), nil
}

type GettingPersonalPromptType string

const (
	GettingPersonalPromptTypeOneThingYouShouldKnow  GettingPersonalPromptType = "oneThingYouShouldKnow"
	GettingPersonalPromptTypeLoveLanguage           GettingPersonalPromptType = "loveLanguage"
	GettingPersonalPromptTypeDorkiestThing          GettingPersonalPromptType = "dorkiestThing"
	GettingPersonalPromptTypeDontHateMeIf           GettingPersonalPromptType = "dontHateMeIf"
	GettingPersonalPromptTypeGeekOutOn              GettingPersonalPromptType = "geekOutOn"
	GettingPersonalPromptTypeIfLovingThisIsWrong    GettingPersonalPromptType = "ifLovingThisIsWrong"
	GettingPersonalPromptTypeKeyToMyHeart           GettingPersonalPromptType = "keyToMyHeart"
	GettingPersonalPromptTypeWontShutUpAbout        GettingPersonalPromptType = "wontShutUpAbout"
	GettingPersonalPromptTypeShouldNotGoOutWithMeIf GettingPersonalPromptType = "shouldNotGoOutWithMeIf"
	GettingPersonalPromptTypeWhatIfIToldYouThat     GettingPersonalPromptType = "whatIfIToldYouThat"
)

func (e *GettingPersonalPromptType) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = GettingPersonalPromptType(s)
	case string:
		*e = GettingPersonalPromptType(s)
	default:
		return fmt.Errorf("unsupported scan type for GettingPersonalPromptType: %T", src)
	}
	return nil
}

type NullGettingPersonalPromptType struct {
	GettingPersonalPromptType GettingPersonalPromptType
	Valid                     bool // Valid is true if GettingPersonalPromptType is not NULL
}

// Scan implements the Scanner interface.
func (ns *NullGettingPersonalPromptType) Scan(value interface{}) error {
	if value == nil {
		ns.GettingPersonalPromptType, ns.Valid = "", false
		return nil
	}
	ns.Valid = true
	return ns.GettingPersonalPromptType.Scan(value)
}

// Value implements the driver Valuer interface.
func (ns NullGettingPersonalPromptType) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return string(ns.GettingPersonalPromptType), nil
}

type MyTypePromptType string

const (
	MyTypePromptTypeNonNegotiable              MyTypePromptType = "nonNegotiable"
	MyTypePromptTypeHallmarkOfGoodRelationship MyTypePromptType = "hallmarkOfGoodRelationship"
	MyTypePromptTypeLookingFor                 MyTypePromptType = "lookingFor"
	MyTypePromptTypeWeirdlyAttractedTo         MyTypePromptType = "weirdlyAttractedTo"
	MyTypePromptTypeAllIAskIsThatYou           MyTypePromptType = "allIAskIsThatYou"
	MyTypePromptTypeWellGetAlongIf             MyTypePromptType = "wellGetAlongIf"
	MyTypePromptTypeWantSomeoneWho             MyTypePromptType = "wantSomeoneWho"
	MyTypePromptTypeGreenFlags                 MyTypePromptType = "greenFlags"
	MyTypePromptTypeSameTypeOfWeird            MyTypePromptType = "sameTypeOfWeird"
	MyTypePromptTypeFallForYouIf               MyTypePromptType = "fallForYouIf"
	MyTypePromptTypeBragAboutYou               MyTypePromptType = "bragAboutYou"
)

func (e *MyTypePromptType) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = MyTypePromptType(s)
	case string:
		*e = MyTypePromptType(s)
	default:
		return fmt.Errorf("unsupported scan type for MyTypePromptType: %T", src)
	}
	return nil
}

type NullMyTypePromptType struct {
	MyTypePromptType MyTypePromptType
	Valid            bool // Valid is true if MyTypePromptType is not NULL
}

// Scan implements the Scanner interface.
func (ns *NullMyTypePromptType) Scan(value interface{}) error {
	if value == nil {
		ns.MyTypePromptType, ns.Valid = "", false
		return nil
	}
	ns.Valid = true
	return ns.MyTypePromptType.Scan(value)
}

// Value implements the driver Valuer interface.
func (ns NullMyTypePromptType) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return string(ns.MyTypePromptType), nil
}

type Religion string

const (
	ReligionAgnostic    Religion = "agnostic"
	ReligionAtheist     Religion = "atheist"
	ReligionBuddhist    Religion = "buddhist"
	ReligionChristian   Religion = "christian"
	ReligionHindu       Religion = "hindu"
	ReligionJain        Religion = "jain"
	ReligionJewish      Religion = "jewish"
	ReligionMuslim      Religion = "muslim"
	ReligionZoroastrian Religion = "zoroastrian"
	ReligionSikh        Religion = "sikh"
	ReligionSpiritual   Religion = "spiritual"
)

func (e *Religion) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = Religion(s)
	case string:
		*e = Religion(s)
	default:
		return fmt.Errorf("unsupported scan type for Religion: %T", src)
	}
	return nil
}

type NullReligion struct {
	Religion Religion
	Valid    bool // Valid is true if Religion is not NULL
}

// Scan implements the Scanner interface.
func (ns *NullReligion) Scan(value interface{}) error {
	if value == nil {
		ns.Religion, ns.Valid = "", false
		return nil
	}
	ns.Valid = true
	return ns.Religion.Scan(value)
}

// Value implements the driver Valuer interface.
func (ns NullReligion) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return string(ns.Religion), nil
}

type StoryTimePromptType string

const (
	StoryTimePromptTypeTwoTruthsAndALie     StoryTimePromptType = "twoTruthsAndALie"
	StoryTimePromptTypeWorstIdea            StoryTimePromptType = "worstIdea"
	StoryTimePromptTypeBiggestRisk          StoryTimePromptType = "biggestRisk"
	StoryTimePromptTypeBiggestDateFail      StoryTimePromptType = "biggestDateFail"
	StoryTimePromptTypeNeverHaveIEver       StoryTimePromptType = "neverHaveIEver"
	StoryTimePromptTypeBestTravelStory      StoryTimePromptType = "bestTravelStory"
	StoryTimePromptTypeWeirdestGift         StoryTimePromptType = "weirdestGift"
	StoryTimePromptTypeMostSpontaneous      StoryTimePromptType = "mostSpontaneous"
	StoryTimePromptTypeOneThingNeverDoAgain StoryTimePromptType = "oneThingNeverDoAgain"
)

func (e *StoryTimePromptType) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = StoryTimePromptType(s)
	case string:
		*e = StoryTimePromptType(s)
	default:
		return fmt.Errorf("unsupported scan type for StoryTimePromptType: %T", src)
	}
	return nil
}

type NullStoryTimePromptType struct {
	StoryTimePromptType StoryTimePromptType
	Valid               bool // Valid is true if StoryTimePromptType is not NULL
}

// Scan implements the Scanner interface.
func (ns *NullStoryTimePromptType) Scan(value interface{}) error {
	if value == nil {
		ns.StoryTimePromptType, ns.Valid = "", false
		return nil
	}
	ns.Valid = true
	return ns.StoryTimePromptType.Scan(value)
}

// Value implements the driver Valuer interface.
func (ns NullStoryTimePromptType) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return string(ns.StoryTimePromptType), nil
}

type UserRole string

const (
	UserRoleUser  UserRole = "user"
	UserRoleAdmin UserRole = "admin"
)

func (e *UserRole) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = UserRole(s)
	case string:
		*e = UserRole(s)
	default:
		return fmt.Errorf("unsupported scan type for UserRole: %T", src)
	}
	return nil
}

type NullUserRole struct {
	UserRole UserRole
	Valid    bool // Valid is true if UserRole is not NULL
}

// Scan implements the Scanner interface.
func (ns *NullUserRole) Scan(value interface{}) error {
	if value == nil {
		ns.UserRole, ns.Valid = "", false
		return nil
	}
	ns.Valid = true
	return ns.UserRole.Scan(value)
}

// Value implements the driver Valuer interface.
func (ns NullUserRole) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return string(ns.UserRole), nil
}

type VerificationStatus string

const (
	VerificationStatusFalse   VerificationStatus = "false"
	VerificationStatusTrue    VerificationStatus = "true"
	VerificationStatusPending VerificationStatus = "pending"
)

func (e *VerificationStatus) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = VerificationStatus(s)
	case string:
		*e = VerificationStatus(s)
	default:
		return fmt.Errorf("unsupported scan type for VerificationStatus: %T", src)
	}
	return nil
}

type NullVerificationStatus struct {
	VerificationStatus VerificationStatus
	Valid              bool // Valid is true if VerificationStatus is not NULL
}

// Scan implements the Scanner interface.
func (ns *NullVerificationStatus) Scan(value interface{}) error {
	if value == nil {
		ns.VerificationStatus, ns.Valid = "", false
		return nil
	}
	ns.Valid = true
	return ns.VerificationStatus.Scan(value)
}

// Value implements the driver Valuer interface.
func (ns NullVerificationStatus) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return string(ns.VerificationStatus), nil
}

type DateVibesPrompt struct {
	ID       int32
	UserID   int32
	Question DateVibesPromptType
	Answer   string
}

type GettingPersonalPrompt struct {
	ID       int32
	UserID   int32
	Question GettingPersonalPromptType
	Answer   string
}

type MyTypePrompt struct {
	ID       int32
	UserID   int32
	Question MyTypePromptType
	Answer   string
}

type Otp struct {
	ID        int32
	UserID    int32
	OtpCode   string
	ExpiresAt pgtype.Timestamptz
}

type StoryTimePrompt struct {
	ID       int32
	UserID   int32
	Question StoryTimePromptType
	Answer   string
}

type User struct {
	ID                  int32
	CreatedAt           pgtype.Timestamptz
	Name                pgtype.Text
	LastName            pgtype.Text
	PhoneNumber         string
	DateOfBirth         pgtype.Date
	Latitude            pgtype.Float8
	Longitude           pgtype.Float8
	Gender              GenderEnum
	DatingIntention     NullDatingIntention
	Height              pgtype.Float8
	Hometown            pgtype.Text
	JobTitle            pgtype.Text
	Education           pgtype.Text
	ReligiousBeliefs    NullReligion
	DrinkingHabit       NullDrinkingSmokingHabits
	SmokingHabit        NullDrinkingSmokingHabits
	MediaUrls           []string
	VerificationStatus  VerificationStatus
	VerificationPic     pgtype.Text
	Role                UserRole
	AudioPromptQuestion NullAudioPrompt
	AudioPromptAnswer   pgtype.Text
}
