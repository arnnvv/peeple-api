package enums

import (
	"database/sql/driver"
	"fmt"
)

type Gender string

const (
	GenderMan      Gender = "man"
	GenderWoman    Gender = "woman"
	GenderBisexual Gender = "bisexual"
	GenderLesbian  Gender = "lesbian"
	GenderGay      Gender = "gay"
)

func ParseGender(s string) (Gender, error) {
	switch s {
	case "man":
		return GenderMan, nil
	case "woman":
		return GenderWoman, nil
	case "bisexual":
		return GenderBisexual, nil
	case "lesbian":
		return GenderLesbian, nil
	case "gay":
		return GenderGay, nil
	default:
		return "", fmt.Errorf("invalid Gender value: %s", s)
	}
}

// DatingIntention enum
type DatingIntention string

const (
	DatingIntentionLifePartner       DatingIntention = "lifePartner"
	DatingIntentionLongTerm          DatingIntention = "longTerm"
	DatingIntentionLongTermOpenShort DatingIntention = "longTermOpenShort"
	DatingIntentionShortTermOpenLong DatingIntention = "shortTermOpenLong"
	DatingIntentionShortTerm         DatingIntention = "shortTerm"
	DatingIntentionFiguringOut       DatingIntention = "figuringOut"
)

func ParseDatingIntention(s string) (DatingIntention, error) {
	switch s {
	case "lifePartner":
		return DatingIntentionLifePartner, nil
	case "longTerm":
		return DatingIntentionLongTerm, nil
	case "longTermOpenShort":
		return DatingIntentionLongTermOpenShort, nil
	case "shortTermOpenLong":
		return DatingIntentionShortTermOpenLong, nil
	case "shortTerm":
		return DatingIntentionShortTerm, nil
	case "figuringOut":
		return DatingIntentionFiguringOut, nil
	default:
		return "", fmt.Errorf("invalid DatingIntention value: %s", s)
	}
}

// Religion enum
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

func ParseReligion(s string) (Religion, error) {
	switch s {
	case "agnostic":
		return ReligionAgnostic, nil
	case "atheist":
		return ReligionAtheist, nil
	case "buddhist":
		return ReligionBuddhist, nil
	case "christian":
		return ReligionChristian, nil
	case "hindu":
		return ReligionHindu, nil
	case "jain":
		return ReligionJain, nil
	case "jewish":
		return ReligionJewish, nil
	case "muslim":
		return ReligionMuslim, nil
	case "zoroastrian":
		return ReligionZoroastrian, nil
	case "sikh":
		return ReligionSikh, nil
	case "spiritual":
		return ReligionSpiritual, nil
	default:
		return "", fmt.Errorf("invalid Religion value: %s", s)
	}
}

// DrinkingSmokingHabits enum
type DrinkingSmokingHabits string

const (
	DrinkingSmokingHabitsYes       DrinkingSmokingHabits = "yes"
	DrinkingSmokingHabitsSometimes DrinkingSmokingHabits = "sometimes"
	DrinkingSmokingHabitsNo        DrinkingSmokingHabits = "no"
)

func ParseDrinkingSmokingHabits(s string) (DrinkingSmokingHabits, error) {
	switch s {
	case "yes":
		return DrinkingSmokingHabitsYes, nil
	case "sometimes":
		return DrinkingSmokingHabitsSometimes, nil
	case "no":
		return DrinkingSmokingHabitsNo, nil
	default:
		return "", fmt.Errorf("invalid DrinkingSmokingHabits value: %s", s)
	}
}

// PromptCategory enum
type PromptCategory string

const (
	PromptCategoryStoryTime       PromptCategory = "storyTime"
	PromptCategoryMyType          PromptCategory = "myType"
	PromptCategoryGettingPersonal PromptCategory = "gettingPersonal"
	PromptCategoryDateVibes       PromptCategory = "dateVibes"
)

func ParsePromptCategory(s string) (PromptCategory, error) {
	switch s {
	case "storyTime":
		return PromptCategoryStoryTime, nil
	case "myType":
		return PromptCategoryMyType, nil
	case "gettingPersonal":
		return PromptCategoryGettingPersonal, nil
	case "dateVibes":
		return PromptCategoryDateVibes, nil
	default:
		return "", fmt.Errorf("invalid PromptCategory value: %s", s)
	}
}

// PromptType enum
type PromptType string

const (
	PromptTypeTwoTruthsAndALie           PromptType = "twoTruthsAndALie"
	PromptTypeWorstIdea                  PromptType = "worstIdea"
	PromptTypeBiggestRisk                PromptType = "biggestRisk"
	PromptTypeBiggestDateFail            PromptType = "biggestDateFail"
	PromptTypeNeverHaveIEver             PromptType = "neverHaveIEver"
	PromptTypeBestTravelStory            PromptType = "bestTravelStory"
	PromptTypeWeirdestGift               PromptType = "weirdestGift"
	PromptTypeMostSpontaneous            PromptType = "mostSpontaneous"
	PromptTypeOneThingNeverDoAgain       PromptType = "oneThingNeverDoAgain"
	PromptTypeNonNegotiable              PromptType = "nonNegotiable"
	PromptTypeHallmarkOfGoodRelationship PromptType = "hallmarkOfGoodRelationship"
	PromptTypeLookingFor                 PromptType = "lookingFor"
	PromptTypeWeirdlyAttractedTo         PromptType = "weirdlyAttractedTo"
	PromptTypeAllIAskIsThatYou           PromptType = "allIAskIsThatYou"
	PromptTypeWellGetAlongIf             PromptType = "wellGetAlongIf"
	PromptTypeWantSomeoneWho             PromptType = "wantSomeoneWho"
	PromptTypeGreenFlags                 PromptType = "greenFlags"
	PromptTypeSameTypeOfWeird            PromptType = "sameTypeOfWeird"
	PromptTypeFallForYouIf               PromptType = "fallForYouIf"
	PromptTypeBragAboutYou               PromptType = "bragAboutYou"
	PromptTypeOneThingYouShouldKnow      PromptType = "oneThingYouShouldKnow"
	PromptTypeLoveLanguage               PromptType = "loveLanguage"
	PromptTypeDorkiestThing              PromptType = "dorkiestThing"
	PromptTypeDontHateMeIf               PromptType = "dontHateMeIf"
	PromptTypeGeekOutOn                  PromptType = "geekOutOn"
	PromptTypeIfLovingThisIsWrong        PromptType = "ifLovingThisIsWrong"
	PromptTypeKeyToMyHeart               PromptType = "keyToMyHeart"
	PromptTypeWontShutUpAbout            PromptType = "wontShutUpAbout"
	PromptTypeShouldNotGoOutWithMeIf     PromptType = "shouldNotGoOutWithMeIf"
	PromptTypeWhatIfIToldYouThat         PromptType = "whatIfIToldYouThat"
	PromptTypeTogetherWeCould            PromptType = "togetherWeCould"
	PromptTypeFirstRoundIsOnMeIf         PromptType = "firstRoundIsOnMeIf"
	PromptTypeWhatIOderForTheTable       PromptType = "whatIOderForTheTable"
	PromptTypeBestSpotInTown             PromptType = "bestSpotInTown"
	PromptTypeBestWayToAskMeOut          PromptType = "bestWayToAskMeOut"
)

func ParsePromptType(s string) (PromptType, error) {
	switch s {
	case "twoTruthsAndALie":
		return PromptTypeTwoTruthsAndALie, nil
	case "worstIdea":
		return PromptTypeWorstIdea, nil
	case "biggestRisk":
		return PromptTypeBiggestRisk, nil
	case "biggestDateFail":
		return PromptTypeBiggestDateFail, nil
	case "neverHaveIEver":
		return PromptTypeNeverHaveIEver, nil
	case "bestTravelStory":
		return PromptTypeBestTravelStory, nil
	case "weirdestGift":
		return PromptTypeWeirdestGift, nil
	case "mostSpontaneous":
		return PromptTypeMostSpontaneous, nil
	case "oneThingNeverDoAgain":
		return PromptTypeOneThingNeverDoAgain, nil
	case "nonNegotiable":
		return PromptTypeNonNegotiable, nil
	case "hallmarkOfGoodRelationship":
		return PromptTypeHallmarkOfGoodRelationship, nil
	case "lookingFor":
		return PromptTypeLookingFor, nil
	case "weirdlyAttractedTo":
		return PromptTypeWeirdlyAttractedTo, nil
	case "allIAskIsThatYou":
		return PromptTypeAllIAskIsThatYou, nil
	case "wellGetAlongIf":
		return PromptTypeWellGetAlongIf, nil
	case "wantSomeoneWho":
		return PromptTypeWantSomeoneWho, nil
	case "greenFlags":
		return PromptTypeGreenFlags, nil
	case "sameTypeOfWeird":
		return PromptTypeSameTypeOfWeird, nil
	case "fallForYouIf":
		return PromptTypeFallForYouIf, nil
	case "bragAboutYou":
		return PromptTypeBragAboutYou, nil
	case "oneThingYouShouldKnow":
		return PromptTypeOneThingYouShouldKnow, nil
	case "loveLanguage":
		return PromptTypeLoveLanguage, nil
	case "dorkiestThing":
		return PromptTypeDorkiestThing, nil
	case "dontHateMeIf":
		return PromptTypeDontHateMeIf, nil
	case "geekOutOn":
		return PromptTypeGeekOutOn, nil
	case "ifLovingThisIsWrong":
		return PromptTypeIfLovingThisIsWrong, nil
	case "keyToMyHeart":
		return PromptTypeKeyToMyHeart, nil
	case "wontShutUpAbout":
		return PromptTypeWontShutUpAbout, nil
	case "shouldNotGoOutWithMeIf":
		return PromptTypeShouldNotGoOutWithMeIf, nil
	case "whatIfIToldYouThat":
		return PromptTypeWhatIfIToldYouThat, nil
	case "togetherWeCould":
		return PromptTypeTogetherWeCould, nil
	case "firstRoundIsOnMeIf":
		return PromptTypeFirstRoundIsOnMeIf, nil
	case "whatIOderForTheTable":
		return PromptTypeWhatIOderForTheTable, nil
	case "bestSpotInTown":
		return PromptTypeBestSpotInTown, nil
	case "bestWayToAskMeOut":
		return PromptTypeBestWayToAskMeOut, nil
	default:
		return "", fmt.Errorf("invalid PromptType value: %s", s)
	}
}

// AudioPrompt enum
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

func ParseAudioPrompt(s string) (AudioPrompt, error) {
	switch s {
	case "canWeTalkAbout":
		return AudioPromptCanWeTalkAbout, nil
	case "captionThisPhoto":
		return AudioPromptCaptionThisPhoto, nil
	case "caughtInTheAct":
		return AudioPromptCaughtInTheAct, nil
	case "changeMyMindAbout":
		return AudioPromptChangeMyMindAbout, nil
	case "chooseOurFirstDate":
		return AudioPromptChooseOurFirstDate, nil
	case "commentIfYouveBeenHere":
		return AudioPromptCommentIfYouveBeenHere, nil
	case "cookWithMe":
		return AudioPromptCookWithMe, nil
	case "datingMeIsLike":
		return AudioPromptDatingMeIsLike, nil
	case "datingMeWillLookLike":
		return AudioPromptDatingMeWillLookLike, nil
	case "doYouAgreeOrDisagreeThat":
		return AudioPromptDoYouAgreeOrDisagreeThat, nil
	case "dontHateMeIfI":
		return AudioPromptDontHateMeIfI, nil
	case "dontJudgeMe":
		return AudioPromptDontJudgeMe, nil
	case "mondaysAmIRight":
		return AudioPromptMondaysAmIRight, nil
	case "aBoundaryOfMineIs":
		return AudioPromptABoundaryOfMineIs, nil
	case "aDailyEssential":
		return AudioPromptADailyEssential, nil
	case "aDreamHomeMustInclude":
		return AudioPromptADreamHomeMustInclude, nil
	case "aFavouriteMemoryOfMine":
		return AudioPromptAFavouriteMemoryOfMine, nil
	case "aFriendsReviewOfMe":
		return AudioPromptAFriendsReviewOfMe, nil
	case "aLifeGoalOfMine":
		return AudioPromptALifeGoalOfMine, nil
	case "aQuickRantAbout":
		return AudioPromptAQuickRantAbout, nil
	case "aRandomFactILoveIs":
		return AudioPromptARandomFactILoveIs, nil
	case "aSpecialTalentOfMine":
		return AudioPromptASpecialTalentOfMine, nil
	case "aThoughtIRecentlyHadInTheShower":
		return AudioPromptAThoughtIRecentlyHadInTheShower, nil
	case "allIAskIsThatYou":
		return AudioPromptAllIAskIsThatYou, nil
	case "guessWhereThisPhotoWasTaken":
		return AudioPromptGuessWhereThisPhotoWasTaken, nil
	case "helpMeIdentifyThisPhotoBomber":
		return AudioPromptHelpMeIdentifyThisPhotoBomber, nil
	case "hiFromMeAndMyPet":
		return AudioPromptHiFromMeAndMyPet, nil
	case "howIFightTheSundayScaries":
		return AudioPromptHowIFightTheSundayScaries, nil
	case "howHistoryWillRememberMe":
		return AudioPromptHowHistoryWillRememberMe, nil
	case "howMyFriendsSeeMe":
		return AudioPromptHowMyFriendsSeeMe, nil
	case "howToPronounceMyName":
		return AudioPromptHowToPronounceMyName, nil
	case "iBeatMyBluesBy":
		return AudioPromptIBeatMyBluesBy, nil
	case "iBetYouCant":
		return AudioPromptIBetYouCant, nil
	case "iCanTeachYouHowTo":
		return AudioPromptICanTeachYouHowTo, nil
	case "iFeelFamousWhen":
		return AudioPromptIFeelFamousWhen, nil
	case "iFeelMostSupportedWhen":
		return AudioPromptIFeelMostSupportedWhen, nil
	default:
		return "", fmt.Errorf("invalid AudioPrompt value: %s", s)
	}
}

type VerificationStatus string

const (
	VerificationStatusFalse   VerificationStatus = "false"
	VerificationStatusTrue    VerificationStatus = "true"
	VerificationStatusPending VerificationStatus = "pending"
)

func (v VerificationStatus) Value() (driver.Value, error) {
	return string(v), nil
}

func (v *VerificationStatus) Scan(value any) error {
	if value == nil {
		*v = VerificationStatusFalse
		return nil
	}

	strValue, ok := value.(string)
	if !ok {
		return fmt.Errorf("invalid value type for VerificationStatus: %T", value)
	}

	*v = VerificationStatus(strValue)
	return nil
}

func (pc *PromptCategory) Scan(value any) error {
	*pc = PromptCategory(value.(string))
	return nil
}

func (pc PromptCategory) Value() (driver.Value, error) {
	return string(pc), nil
}

// Add to AudioPrompt
func (ap *AudioPrompt) Scan(value any) error {
	*ap = AudioPrompt(value.(string))
	return nil
}

func (ap AudioPrompt) Value() (driver.Value, error) {
	return string(ap), nil
}

func (g *Gender) Scan(value any) error {
	*g = Gender(value.(string))
	return nil
}

func (g Gender) Value() (driver.Value, error) {
	return string(g), nil
}

// DatingIntention methods
func (d *DatingIntention) Scan(value any) error {
	*d = DatingIntention(value.(string))
	return nil
}

func (d DatingIntention) Value() (driver.Value, error) {
	return string(d), nil
}

// Religion methods
func (r *Religion) Scan(value any) error {
	*r = Religion(value.(string))
	return nil
}

func (r Religion) Value() (driver.Value, error) {
	return string(r), nil
}

// DrinkingSmokingHabits methods
func (h *DrinkingSmokingHabits) Scan(value any) error {
	*h = DrinkingSmokingHabits(value.(string))
	return nil
}

func (h DrinkingSmokingHabits) Value() (driver.Value, error) {
	return string(h), nil
}
