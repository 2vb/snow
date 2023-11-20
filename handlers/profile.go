package handlers

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ectrc/snow/aid"
	p "github.com/ectrc/snow/person"

	"github.com/gofiber/fiber/v2"
)

var (
	profileActions = map[string]func(c *fiber.Ctx, person *p.Person, profile *p.Profile) error {
		"QueryProfile": PostQueryProfileAction,
		"ClientQuestLogin": PostQueryProfileAction,
		"MarkItemSeen": PostMarkItemSeenAction,
		"EquipBattleRoyaleCustomization": PostEquipBattleRoyaleCustomizationAction,
		"SetBattleRoyaleBanner": PostSetBattleRoyaleBannerAction,
	}
)

func PostProfileAction(c *fiber.Ctx) error {
	person := p.Find(c.Params("accountId"))
	if person == nil {
		return c.Status(404).JSON(aid.ErrorBadRequest("No Account Found"))
	}

	profile := person.GetProfileFromType(c.Query("profileId"))
	if action, ok := profileActions[c.Params("action")]; ok && profile != nil {
		defer profile.ClearProfileChanges()
		before := profile.Snapshot()
		if err := action(c, person, profile); err != nil {
			return c.Status(400).JSON(aid.ErrorBadRequest(err.Error()))
		}
		profile.Diff(before)
	}
	
	revision, _ := strconv.Atoi(c.Query("rvn"))
	if revision == -1 && profile == nil {
		revision = 1
	}
	if revision == -1 && profile != nil {
		revision = profile.Revision
	}
	revision++

	changes := []interface{}{}
	if profile != nil {
		changes = profile.Changes
	}

	return c.Status(200).JSON(aid.JSON{
		"profileId": c.Query("profileId"),
		"profileRevision": revision,
		"profileCommandRevision": revision,
		"profileChangesBaseRevision": revision - 1,
		"profileChanges": changes,
		"multiUpdate": []aid.JSON{},
		"notifications": []aid.JSON{},
		"responseVersion": 1,
		"serverTime": time.Now().Format("2006-01-02T15:04:05.999Z"),
	})
}

func PostQueryProfileAction(c *fiber.Ctx, person *p.Person, profile *p.Profile) error {
	profile.CreateFullProfileUpdateChange()
	return nil
}

func PostMarkItemSeenAction(c *fiber.Ctx, person *p.Person, profile *p.Profile) error {
	var body struct {
		ItemIds []string `json:"itemIds"`
	}

	err := c.BodyParser(&body)
	if err != nil {
		return fmt.Errorf("invalid Body")
	}

	for _, itemId := range body.ItemIds {
		item := profile.Items.GetItem(itemId)
		if item == nil {
			continue
		}
		
		item.HasSeen = true
		go item.Save()
	}

	return nil
}

func PostEquipBattleRoyaleCustomizationAction(c *fiber.Ctx, person *p.Person, profile *p.Profile) error {
	var body struct {
		SlotName string `json:"slotName" binding:"required"`
		ItemToSlot string `json:"itemToSlot"`
		IndexWithinSlot int `json:"indexWithinSlot"`
	}

	err := c.BodyParser(&body)
	if err != nil {
		return fmt.Errorf("invalid Body")
	}

	item := profile.Items.GetItem(body.ItemToSlot)
	if item == nil {
		if body.ItemToSlot != "" && !strings.Contains(strings.ToLower(body.ItemToSlot), "random") {
			return fmt.Errorf("item not found")
		}

		item = &p.Item{
			ID: body.ItemToSlot,
		}
	}

	attr := profile.Attributes.GetAttributeByKey("favorite_" + strings.ToLower(body.SlotName))
	if attr == nil {
		return fmt.Errorf("attribute not found")
	}

	switch body.SlotName {
	case "Dance":
		value := aid.JSONParse(attr.ValueJSON)
		value.([]any)[body.IndexWithinSlot] = item.ID
		attr.ValueJSON = aid.JSONStringify(value)
	case "ItemWrap":
		value := aid.JSONParse(attr.ValueJSON)
		value.([]any)[body.IndexWithinSlot] = item.ID
		attr.ValueJSON = aid.JSONStringify(value)
	default:
		attr.ValueJSON = aid.JSONStringify(item.ID)
	}

	go attr.Save()
	return nil
}

func PostSetBattleRoyaleBannerAction(c *fiber.Ctx, person *p.Person, profile *p.Profile) error {
	var body struct {
		HomebaseBannerColorID string `json:"homebaseBannerColorId" binding:"required"`
		HomebaseBannerIconID string `json:"homebaseBannerIconId" binding:"required"`
	}
	
	err := c.BodyParser(&body)
	if err != nil {
		return fmt.Errorf("invalid Body")
	}

	colorItem := person.CommonCoreProfile.Items.GetItemByTemplateID("HomebaseBannerColor:"+body.HomebaseBannerColorID)
	if colorItem == nil {
		return fmt.Errorf("color item not found")
	}

	iconItem := person.CommonCoreProfile.Items.GetItemByTemplateID("HomebaseBannerIcon:"+body.HomebaseBannerIconID)
	if iconItem == nil {
		return fmt.Errorf("icon item not found")
	}

	iconAttr := profile.Attributes.GetAttributeByKey("banner_icon")
	if iconAttr == nil {
		return fmt.Errorf("icon attribute not found")
	}

	colorAttr := profile.Attributes.GetAttributeByKey("banner_color")
	if colorAttr == nil {
		return fmt.Errorf("color attribute not found")
	}

	iconAttr.ValueJSON = aid.JSONStringify(strings.Split(iconItem.TemplateID, ":")[1])
	colorAttr.ValueJSON = aid.JSONStringify(strings.Split(colorItem.TemplateID, ":")[1])

	go func() {
		iconAttr.Save()
		colorAttr.Save()
	}()
	return nil
}