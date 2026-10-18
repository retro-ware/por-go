package engine

// itemType is an item's base type: the Type of an item record and the
// index into the ITEMS table (itemBase). Weapons 0x01..0x2F follow the
// order of their names in itemWords; the rest are named after the items of
// that type in the game's ITEM and MONnITM blocks. Only the types the code
// tests are named.
type itemType byte

const (
	itemHandAxe           itemType = 0x02
	itemClub              itemType = 0x07
	itemDagger            itemType = 0x08
	itemDart              itemType = 0x09
	itemHammer            itemType = 0x14
	itemJavelin           itemType = 0x15
	itemQuarrel           itemType = 0x1C
	itemSpear             itemType = 0x1F
	itemBastardSword      itemType = 0x22
	itemBroadSword        itemType = 0x23
	itemLongSword         itemType = 0x24
	itemShortSword        itemType = 0x25
	itemTwoHandedSword    itemType = 0x26
	itemCompositeLongBow  itemType = 0x29
	itemCompositeShortBow itemType = 0x2A
	itemLongBow           itemType = 0x2B
	itemShortBow          itemType = 0x2C
	itemSling             itemType = 0x2F
	itemLeatherArmor      itemType = 0x32
	itemPaddedArmor       itemType = 0x33
	itemStuddedLeather    itemType = 0x34
	itemRingMail          itemType = 0x35
	itemScaleMail         itemType = 0x36
	itemChainMail         itemType = 0x37
	itemSplintMail        itemType = 0x38
	itemBandedMail        itemType = 0x39
	itemPlateMail         itemType = 0x3A
	itemShield            itemType = 0x3B
	itemMageScroll        itemType = 0x3D
	itemClericScroll      itemType = 0x3E
	itemGauntlets         itemType = 0x3F // of Ogre Power
	itemCloak             itemType = 0x43
	itemRing              itemType = 0x45
	itemValuable          itemType = 0x46 // jewellery, manuals, curios
	itemPotion            itemType = 0x47 // also holy symbols
	itemArrow             itemType = 0x49
	itemBracers           itemType = 0x4D
	itemWand              itemType = 0x4E
	itemWandOfMissiles    itemType = 0x4F
	itemStrengthPotion    itemType = 0x54 // of Giant Strength
	itemHolyWater         itemType = 0x55
	itemOil               itemType = 0x56
	itemBoulder           itemType = 0x57
	itemGiantBoulder      itemType = 0x58 // 1d12+8, range 20; no item record has it
	itemCloakDisplacement itemType = 0x5C
	itemProtectionRing    itemType = 0x5D
)
