package constants

import amqp "github.com/kaellybot/kaelly-amqp"

// GetItemTypesBySuperType maps Dofus Touch item super types to the bot's item types.
// A super type missing here is not served (cubes, gems, breloques, buffs...).
// Equipment types (hat, ring...) are mapped in phase 2.
func GetItemTypesBySuperType() map[int32]amqp.ItemType {
	return map[int32]amqp.ItemType{
		1:  amqp.ItemType_EQUIPMENT_TYPE,  // Amulets
		2:  amqp.ItemType_EQUIPMENT_TYPE,  // Weapons
		3:  amqp.ItemType_EQUIPMENT_TYPE,  // Rings
		4:  amqp.ItemType_EQUIPMENT_TYPE,  // Belts
		5:  amqp.ItemType_EQUIPMENT_TYPE,  // Boots
		6:  amqp.ItemType_CONSUMABLE_TYPE, // Consumables
		7:  amqp.ItemType_EQUIPMENT_TYPE,  // Shields
		9:  amqp.ItemType_RESOURCE_TYPE,   // Resources
		10: amqp.ItemType_EQUIPMENT_TYPE,  // Hats
		11: amqp.ItemType_EQUIPMENT_TYPE,  // Cloaks, backpacks
		12: amqp.ItemType_EQUIPMENT_TYPE,  // Pets, petsmounts
		13: amqp.ItemType_EQUIPMENT_TYPE,  // Trophies, Dofus
		14: amqp.ItemType_QUEST_ITEM_TYPE, // Quest items
		16: amqp.ItemType_CONSUMABLE_TYPE, // Boost food
		21: amqp.ItemType_MOUNT_TYPE,      // Mounts
		22: amqp.ItemType_COSMETIC_TYPE,   // Living objects
		23: amqp.ItemType_COSMETIC_TYPE,   // Ceremonial weapons, soul stones
		24: amqp.ItemType_COSMETIC_TYPE,   // Ceremonial shields
		26: amqp.ItemType_COSMETIC_TYPE,   // Ceremonial hats
		27: amqp.ItemType_COSMETIC_TYPE,   // Ceremonial cloaks
		28: amqp.ItemType_COSMETIC_TYPE,   // Ceremonial pets
		43: amqp.ItemType_CONSUMABLE_TYPE, // Brews
		49: amqp.ItemType_CONSUMABLE_TYPE, // Serums, usable objects
	}
}

// GetLanguages maps the client data languages to the bot's locales.
func GetLanguages() map[string]amqp.Language {
	return map[string]amqp.Language{
		"fr": amqp.Language_FR,
		"en": amqp.Language_EN,
		"es": amqp.Language_ES,
		"de": amqp.Language_DE,
		"pt": amqp.Language_PT,
	}
}
