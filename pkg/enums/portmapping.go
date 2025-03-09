package enums

import "slices"

func (pc PromptCategory) GetPrompts() []PromptType {
	switch pc {
	case PromptCategoryStoryTime:
		return []PromptType{
			PromptTypeTwoTruthsAndALie,
			PromptTypeWorstIdea,
			PromptTypeBiggestRisk,
			PromptTypeBiggestDateFail,
			PromptTypeNeverHaveIEver,
			PromptTypeBestTravelStory,
			PromptTypeWeirdestGift,
			PromptTypeMostSpontaneous,
			PromptTypeOneThingNeverDoAgain,
		}

	case PromptCategoryMyType:
		return []PromptType{
			PromptTypeNonNegotiable,
			PromptTypeHallmarkOfGoodRelationship,
			PromptTypeLookingFor,
			PromptTypeWeirdlyAttractedTo,
			PromptTypeAllIAskIsThatYou,
			PromptTypeWellGetAlongIf,
			PromptTypeWantSomeoneWho,
			PromptTypeGreenFlags,
			PromptTypeSameTypeOfWeird,
			PromptTypeFallForYouIf,
			PromptTypeBragAboutYou,
		}

	case PromptCategoryGettingPersonal:
		return []PromptType{
			PromptTypeOneThingYouShouldKnow,
			PromptTypeLoveLanguage,
			PromptTypeDorkiestThing,
			PromptTypeDontHateMeIf,
			PromptTypeGeekOutOn,
			PromptTypeIfLovingThisIsWrong,
			PromptTypeKeyToMyHeart,
			PromptTypeWontShutUpAbout,
			PromptTypeShouldNotGoOutWithMeIf,
			PromptTypeWhatIfIToldYouThat,
		}

	case PromptCategoryDateVibes:
		return []PromptType{
			PromptTypeTogetherWeCould,
			PromptTypeFirstRoundIsOnMeIf,
			PromptTypeWhatIOderForTheTable,
			PromptTypeBestSpotInTown,
			PromptTypeBestWayToAskMeOut,
		}

	default:
		return []PromptType{}
	}
}

func (pt PromptType) GetCategory() PromptCategory {
	for _, category := range []PromptCategory{
		PromptCategoryStoryTime,
		PromptCategoryMyType,
		PromptCategoryGettingPersonal,
		PromptCategoryDateVibes,
	} {
		prompts := category.GetPrompts()
		if slices.Contains(prompts, pt) {
			return category
		}
	}
	return PromptCategoryStoryTime
}
