package flow

import (
	"time"

	"github.com/jinzhu/gorm"
)

// SfWorkFlows This table holds the definition for each workflow process, such as 'Order Fulfillment'.
type SfWorkFlows struct {
	gorm.Model
	Name        string          `gorm:"type:varchar(100)"`
	Description string          `gorm:"type:text"`
	IsValid     bool            `gorm:"default:false"`
	IsDraft     bool            `gorm:"default:true"`
	ErrorMsg    string          `gorm:"type:text"`
	Places      []SfPlaces      `gorm:"foreignkey:WorkflowID"`
	Transitions []SfTransitions `gorm:"foreignkey:WorkflowID"`
	Arcs        []SfArcs        `gorm:"foreignkey:WorkflowID"`
	Cases       []SfCases       `gorm:"foreignkey:WorkflowID"`
	Tokens      []SfTokens      `gorm:"foreignkey:WorkflowID"`
}

// SfPlaces This table holds the details for each place within a workflow process.
type SfPlaces struct {
	gorm.Model
	WorkflowID  uint        `gorm:"type:bigint"`
	Name        string      `gorm:"type:text"`
	Description string      `gorm:"type:text"`
	SortOrder   uint        `gorm:"default:0"`
	PlaceType   TypePlace   `gorm:"default:0"`
	WorkFlow    SfWorkFlows `gorm:"foreignkey:WorkflowID"`
	Arcs        []SfArcs    `gorm:"foreignkey:PlaceID"`
	Tokens      []SfTokens  `gorm:"foreignkey:PlaceID"`
	ArcGurander ArcGurand   `gorm:"-"`
}

// SfTransitions his table holds the details for each transition within a workflow process,
// such as 'Charge Customer', 'Pack Order' and 'Ship Order'.
// Each record will point to an application task within the MENU database.
type SfTransitions struct {
	gorm.Model
	WorkflowID         uint          `gorm:"type:bigint"`
	Name               string        `gorm:"type:varchar(100)"`
	Description        string        `gorm:"type:text"`
	SortOrder          uint          `gorm:"default:0"`
	WorkFlow           SfWorkFlows   `gorm:"foreignkey:WorkflowID"`
	Arcs               []SfArcs      `gorm:"foreignkey:TransitionID"`
	TiggerLimit        uint          `gorm:"type:int"`
	TiggerType         uint          `gorm:"default:0"`
	TransitionCallback []interface{} `gorm:"-"`
}

// SfArcs This table holds the details for each arc within a workflow process.
// An arc links a place to a transition.
type SfArcs struct {
	gorm.Model
	WorkflowID   uint    `gorm:"type:bigint"`
	TransitionID uint    `gorm:"type:bigint"`
	PlaceID      uint    `gorm:"type:bigint"`
	Direction    uint    `gorm:"default:0"`
	ArcType      TypeArc `gorm:"type:varchar(50)"`
}

// SfCases This identifies when a particular instance of a workflow was started,
// its context and its current status.
type SfCases struct {
	gorm.Model
	WorkflowID uint        `gorm:"type:bigint"`
	State      CaseStatus  `gorm:"type:varchar(100)"`
	Workflow   SfWorkFlows `gorm:"foreignkey:WorkflowID"`
	Tokens     []SfTokens  `gorm:"foreignkey:CaseID"`
	StartAt    *time.Time
	EndAt      *time.Time
}

// SfTokens This identifies when a token was inserted into a particular place.
type SfTokens struct {
	gorm.Model
	WorkflowID       uint        `gorm:"type:bigint"`
	CaseID           uint        `gorm:"type:bigint"`
	PlaceID          uint        `gorm:"type:bigint"`
	State            TokenStatus `gorm:"type:varchar(50)"`
	WorkItemID       uint        `gorm:"type:bigint"`
	LockedWorkItemID uint        `gorm:"type:bigint"`
	WorkFlow         SfWorkFlows `gorm:"foreignkey:WorkflowID"`
	Case             SfCases     `gorm:"foreignkey:CaseID"`
	Place            SfPlaces    `gorm:"foreignkey:PlaceID"`
	ProducedAt       *time.Time  `gorm:"default:CURRENT_TIMESTAMP"`
	LockedAt         *time.Time
	CanceledAt       *time.Time
	ConsumedAt       *time.Time
}

// AfterCreate on token create
// Get a list of all places which input to the transition.
// Check that each of these places has a token (an AND-join must have one token for each input place). If the correct number of tokens is found then
// 1. enable the transition (create a new WORKITEM record) AND fire thre translation tigger
// 2. If the transition trigger is 'AUTO' then process that TASK immediately, otherwise wait for some other trigger.
func (token *SfTokens) AfterCreate(tx *gorm.DB) (err error) {
	err = afterCreateOrUpdateToken(tx, token)
	return err
}

// AfterUpdate same of create
func (token *SfTokens) AfterUpdate(tx *gorm.DB) (err error) {
	err = afterCreateOrUpdateToken(tx, token)
	return err
}

func afterCreateOrUpdateToken(tx *gorm.DB, token *SfTokens) (err error) {
	tx.Model(&token).Related(&token.Case)
	tx.Model(&token).Related(&token.Place)
	if token.Place.PlaceType == END {
		token.Case.State = CLOSE
		tx.Save(&token.Case)
		return
	}
	// Find all inward arcs which go from this place
	// Note Place -out-> Transition -in-> Place
	var inwardArcs SfArcs
	var outputTransition SfTransitions
	tx.Where("direction = ?", OUT).Where("place_id", token.Place.ID).First(&inwardArcs)
	tx.First(&outputTransition, inwardArcs.TransitionID)
	// if AND-JOIN type of arc ,check all place token
	if inwardArcs.ArcType == ANDJ {
		var inputPlacesIDs []uint
		var inputTokenNum uint
		tx.Model(&SfArcs{}).Where("direction = ?", OUT).Where("transition_id = ?", outputTransition.ID).Pluck("place_id", inputPlacesIDs)
		tx.Model(&SfTokens{}).Where("case_id = ?", token.CaseID).Where("place_id IN ?", inputPlacesIDs).Where("state", LOCK).Count(&inputTokenNum)
		if len(inputPlacesIDs) == int(inputTokenNum) {
			// TODO Call Transitions
		}
		return
	}
	return
}
