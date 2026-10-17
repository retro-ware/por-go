# Xeen Event Opcodes Reference

Source: https://xeen.fandom.com/wiki/Category:Event_Opcodes (fetched 2026-10-05)

# Event Scripting

*Events* are scripted interactions that can be placed at any position on any map. They can take a variety of inputs, with access to input from the player in the form of getting them to enter a word or phrase, choose from a number of pre-set options, or selecting a character. Events can modify a wide variety of in-game variables: character stats, health, conditions, party gold, gems, awards, and so forth. They can also modify the map by adding monsters, changing tiles, adding objects, removing objects, and removing events, as well as providing output to the player in the form of NPC dialogs and output windows full of text, playing sound effects or CD audio, etc.

The events for a given map are stored in a file named MAZExxxx.EVT, where xxxx is a decimal representation of the map number. Like all other map files used by the Xeen engine, the first zero is replaced by an 'X' if the map number is greater than 99: MAZE0024.EVT for map number 24, and MAZEX128.EVT for map number 128.

Each individual event is stored as a set of *lines*, each of which contains an instruction for the engine to execute. The lines are numbered, so that it is possible to jump between lines using the equivalent of a GOTO command.

The event file itself consists of these lines all stored one after the other. Each line consists of:
* Length byte - the next *n* bytes make up this line
* byte x - the X-offset of this event
* byte y - the Y-offset of this event
* byte facing - the directions this event can be triggered from
* Line number (byte) - the relative number of this line in the current event
* Opcode byte - the command to execute
* Optional arguments - any number of bytes

The X, Y, and facing bytes combine to indicate the tile the party must be standing on, and the direction they must be facing to trigger the event. A direction value of 4 means that the event can be triggered if the party is facing any direction.

== Event Example == 

The raw data from the Vertigo event file has been broken down to show the line structure:

```
08  10 08 04  00     09  14 F5 02
08  10 08 04  01     08  09 00 05
06  10 08 04  02     01  1C 
08  10 08 04  03     09  2C 01 04
05  10 08 04  04     12 
07  10 08 04  05     20  00 28
0E  10 08 04  06     14  64 0A 00 00 00 00 00 00 00
09  10 08 04  07     0C  00 00 14 F5 
```

The first byte of each line shows the length of the line data (the number of following bytes.

The next three bytes are the X, Y, and facing bytes: this event is located at X=16, Y=8, and can be triggered from any direction.

The next byte is the line number. This is always incremented by one in every original Xeen game event examined.

There is always at least one more byte: the opcode for the command the engine should perform, which may have data bytes. In the above example, all the lines except line 4 (opcode 0x12) have at least one data byte.

```
00: If                 14 F5 02
01: If                 09 00 05
02: Display0x01        1C 
03: If                 2C 01 04
04: Exit 
05: WhoWill            00 28
06: GiveTreasureChest  64 0A 00 00 00 00 00 00 00
07: TakeOrGive         00 00 14 F5 
```

The game engine stores the results of certain commands in memory. The exact handling of the parameters for each of the above lines is available on each command's individual page, however a brief overview of this event is:

* 0. If map flag F5 is set, go to line 2.
** 1. Otherwise, go to line 5.
* 2. Show the "Nothing here." message.
* 3. Wait for the user to press a key.
* 4. Stop execution of the script.
* 5. Ask the user "Who will search?". If the user presses escape, the event will exit, otherwise execution will continue to the next line when they select a character.
* 6. Use the GiveTreasureChest function to give the party a random amount of up to 10 gold.
* 7. Set map flag F5, so that the user can't search this tree again. (See line 1)
* There are no more lines, so the event is finished.

As you can see, many commands (such as If) are capable of serving multiple purposes, and can do many different things depending on the parameters passed.

#### All Opcodes

| Opcode | Opcode Name | Arguments |
|---|---|---|
| 0x01 | Display0x01 | *string* text |
| 0x02 | DoorTextSml | *string* text |
| 0x03 | DoorTextLrg | *string* text |
| 0x04 | SignText | *string* text |
| 0x05 | NPC | *string* name, *string* conversationText, *byte* faceNumber, *byte* confirm, *optional lineNumber* nextLineNumber |
| 0x06 | PlayFX | *byte* effectId |
| 0x07 | TeleportAndExit | *byte* mapId, *byte* x, *byte* y |
| 0x08 | If | *byte* comparisonType, *itemdata* value, *lineNumber* destination |
| 0x09 | If | *byte* comparisonType, *itemdata* value, *lineNumber* destination |
| 0x0A | If | *byte* comparisonType, *itemdata* value, *lineNumber* destination |
| 0x0B | MoveObj | *byte* objectNumber, *byte* x, *byte* y |
| 0x0C | TakeOrGive | *byte* takeType, *itemdata* object1, *byte* giveType, *itemdata* object2 |
| 0x0D | NoAction | - |
| 0x0E | Remove | - |
| 0x0F | SetChar | *byte* characterIndex |
| 0x10 | Spawn | *byte* monsterId, *byte* x, *byte* y, *byte* unknown |
| 0x11 | DoTownEvent | *byte* townEventId |
| 0x12 | Exit | - |
| 0x13 | AlterMap | *byte* x, *byte* y, *byte* wall, *byte* value |
| 0x14 | GiveExtended | *unknown* |
| 0x15 | ConfirmWord | *string* acceptableWord1, *lineNumber* destination, *string* acceptableWord2, *string* promptText |
| 0x16 | Damage | *uint16* amount, *byte* type |
| 0x17 | JumpRnd | *byte* max, *byte* comparison, *lineNumber* destination |
| 0x18 | AlterEvent | *lineNumber* lineNumber, *byte* newOpcode |
| 0x19 | CallEvent | *byte* x, *byte* y, *byte* facingDirection |
| 0x1A | Return | - |
| 0x1B | SetVar | *byte* variableId, *itemdata* newValue |
| 0x1C | TakeOrGive | *byte* giveType, *itemdata* compareValue, *byte* giveType, *itemdata* object2 |
| 0x1D | TakeOrGive | *byte* takeType, *itemdata* object1, *byte* giveType, *itemdata* object2 |
| 0x1E | CutsceneEndClouds | - |
| 0x1F | TeleportAndContinue | *byte* mapId, *byte* x, *byte* y |
| 0x20 | WhoWill | *byte* whatIndex, *string* descriptionText |
| 0x21 | RndDamage | *byte* damageType, *byte* max |
| 0x22 | MoveWallObj | *byte* wallObjectNumber, *byte* x, *byte* y |
| 0x23 | AlterCellFlag | *byte* x, *byte* y, *byte* newFlagValue |
| 0x24 | AlterHed | *uint16* newValue |
| 0x25 | DisplayStat | *string* text |
| 0x26 | TakeOrGive | *byte* giveType, *itemdata* object1, *byte* giveType, *itemdata* object2 |
| 0x27 | SeatTextSml | *string* text |
| 0x28 | PlayEventVoc | *byte* vocIndex |
| 0x29 | DisplayBottom | *string* descriptionText |
| 0x2A | IfMapFlag | *byte* monsterId, *lineNumber* destination |
| 0x2B | SelRndChar | - |
| 0x2C | GiveEnchanted | *byte* itemType, *byte* baseEnchantment, *byte* extraEnchantment, *byte* unknown |
| 0x2D | ItemType | *byte* itemType |
| 0x2E | MakeNothingHere | - |
| 0x2F | NoAction | - |
| 0x30 | ChooseNumeric | *byte* numberOfOptions, *byte*n* arrayOfLineNumbers |
| 0x31 | DisplayBottomTwoLines | *string* firstLine, *string* secondLine |
| 0x32 | DisplayLarge | *string* text |
| 0x33 | ExchObj | *byte* objectNumber1, *byte* objectNumber2 |
| 0x34 | FallToMap | *map* mapId, *byte* x, *byte* y, *byte* fallDamage |
| 0x35 | DisplayMain | *string* text |
| 0x36 | Goto | *byte* c_surface, *lineNumber* destination |
| 0x37 | ConfirmWord | *string* acceptableWord1, *lineNumber* destination1, *string* acceptableWord2, *lineNumber* destination2 |
| 0x38 | GotoRandom | *byte* numberOfLineNumbers, *byte*n* arrayOfLineNumbers |
| 0x39 | CutsceneEndDarkside | - |
| 0x3a | CutsceneEndWorld | - |
| 0x3b | FlipWorld | *byte* sideFlag |
| 0x3c | PlayCD | *byte* cdTrackNumber, *uint16* startFrame, *uint16* endFrame |

Category:Engine

# Opcodes

## 0x00 - NOP

**Arguments**

none

**Description**

```
Does nothing
```

**Notes**

```
Does nothing.  Rarely appears in scripts.  Is used most often to replace If conditions via the AlterEvent instruction.  NOP stands for No OPeration.
```

## 0x01 - Display

**Arguments**

* string (index into map text file) - String to display

**Description**

Displays a small message to the user, often used as a prompt to various Yes/No questions.

**Notes**

Message overlays the arrow buttons in the lower right corner.  For example, see the element machines at the four corners of Clouds.

## 0x02 - DoorTextSml

**Arguments**

* string (index into map text file)

**Description**

right
This operation draws the supplied string in the 3D game window. The text is drawn using the small font set, formatted and aligned so that it would appear over the top of a door, and this opcode is only ever used in events directly in front of a door sprite. A typical example can be seen in the image to the right.

This is one of the few events that is automatically executed and does not need to be triggered by the player pressing the spacebar to interact with the event/object.

**Notes**

```
The functionality of this opcode is identical to DoorTextLrg, except that it draws the text using the small font rather than the large font. See also SeatTextSml and SeatTextLrg.
```

## 0x03 - DoorTextLrg

**Arguments**

* string (index into map text file)

**Description**

right
This operation draws the supplied string in the 3D game window. The text is drawn using the large font set, formatted and aligned so that it would appear over the top of a door. This opcode would only ever be used in events directly in front of a door sprite. An example can be seen in the image to the right.

This is one of the few events that is automatically executed and does not need to be triggered by the player pressing the spacebar to interact with the event/object.

**Notes**

```
The functionality of this opcode is identical to DoorTextSml, except that it draws the text using the large font rather than the small font. See also SeatTextSml and SeatTextLrg.
```

Matt does not recall having ever seen this opcode used in any game, but needs to do a thorough search before this can be stated as fact.

## 0x04 - SignTextLrg

**Arguments**

* string (index into map text file)

**Description**

right
This operation draws the supplied string in the 3D game window. The text is drawn using the large font set, formatted and aligned so that it would appear over the top of a seat sprite. This opcode would only ever be used in events directly in front of a seat sprite. An example can be seen in the image to the right.

This is one of the few events that is automatically executed and does not need to be triggered by the player pressing the spacebar to interact with the event/object.

**Notes**

```
The functionality of this opcode is identical to SeatTextSml, except that it draws the text using the large font rather than the small font. See also DoorTextSml and DoorTextLrg.
```

## 0x05 - NPC

**Arguments**

* string (index into map text file) - NPC Name
* string (index into map text file) - Conversation text
* byte - NPC portrait number (FACExx.FAC)
* byte - Confirm
* lineNumber - *Optional* next line number

**Description**

Displays the standard NPC interaction window, as shown below. The NPC name and the conversation text are read from the map's text file, and the specified NPC portrait is loaded and animated as well. If the Confirm byte is 0, yes/no confirmation buttons are displayed.  If the player selects "yes", execution is moved to the given line number, otherwise execution continues on the next line.  If the Confirm byte is 1, the next line value is ignored, and execution continues on the next line always.  Confirm byte of 2 seems to be a special case that requires more investigation.

center

#### Notes:
Used in connection with 0x3C (60) PlayCD Opcode on the World of Xeen CD edition, and it will cause cross-talk issues if a PlayCD Opcode is not used before the NPC Opcode. For example if you hear vocals from someone you talked to in town, then type in Warzone at the mirror and do an Arena Event you will hear the same vocals again because the arena event doesn't have a PlayCD Opcode.

## 0x06 - PlayFX

**Arguments**

* Byte - Effect number to play

**Description**

Instructs the music driver to play the given effect number.

**Notes**

The effect numbers are hard coded as part of the music drivers.

## 0x07/0x1F - Teleport

**Arguments**

* byte mapId - Destination Map ID
* byte x
* byte y

**Description**

Teleports the party to the specified destination and stops execution of the current script.

**Notes**

This function can accept a destination map ID of zero, in which case the results of previous operations are used (potentially for a mirror teleport lookup.) This needs further examination.

Opcode 0x07 exits the event following a successful teleport.  Opcode 0x1F continues with the next instruction.

## 0x08/0x09/0x0A - If

**Arguments**

* byte - comparison type
* Additional arguments by type
* lineNumber - destination line number if comparison is true

**Description**

Performs a comparison depending on type and additional bytes.  Jumps to the given line number if comparison is true, else continues to the next line.  Most comparisons are performed on the currently selected character, as by WhoWill or SelRndChar for example.
* 0x08 - true if actual value <= comparison value
* 0x09 - true if actual value == comparison value
* 0x0A - true if actual value > comparison value

**Notes**

The "If" structure is very complex, with many, many comparison types.  It always takes at least 3 arguments: the comparison type; at least one argument (though may be more, depending on type); and a destination number.  This will need a lot of further investigation.

08 09 00 is often used as always true: every character will have at least 0 spell points. (if sp >= 0, do...)

| Type | Arguments | Description |
|---|---|---|
| 0x00 | --- | UNUSED: This does nothing. Do Not use! |
| 0x01 | --- | UNUSED: This does nothing. Do Not use! |
| 0x02 | --- | UNUSED: This does nothing. Do Not use! |
| 0x03 | Sex | TODO: Confirm 0=male, 1=female |
| 0x04 | Race | 0=Human, 1=Elf, 2=Dwarf, 3=Gnome, 4=H-Orc |
| 0x05 | Class | 0=Knight, 1=Paladin, 2=Archer, 3=Cleric, 4=Sorcerer, 5=Robber, 6=Ninja, 7=Barbarian, 8=Druid, 9=Ranger |
| 0x06 | --- | UNUSED: This does nothing. Do Not use! |
| 0x07 | --- | UNUSED: This does nothing. Do Not use! |
| 0x08 | HP (1) | Current Health Points. TODO: Why is this only 1 byte comparison?  It can go higher than 256. |
| 0x09 | SP (1) | Current Spell Points. |
| 0x0A | AC (1) | Current Armor Class, including bonuses. TODO: Why is this only 1 byte comparison?  It can go higher than 256. |
| 0x0B | LevelTmp (1) | Level bonus (extra beyond base) |
| 0x0C | Age (1) | Current Age, including unnatural aging.  Capped at 254. |
| 0x0D | Skill (1) | Has the skill. 0=Theivary, 1=ArmsMaster, 2=Astrologer, 3=BodyBuilder, 4=Cartographer, 5=Crusader, 6=DirectionSense, 7=Linguist, 8=Merchant, 9=Mountaineer, 10=Navigator, 11=PathFinder, 12=PrayerMaster, 13=Prestidigitator, 14=Swimmer, 15=Tracker, 16=SpotSecretDoors, 17=DangerSense |
| 0x0E | --- | UNUSED: This does nothing. Do Not use! |
| 0x0F | Award | Checks if you have the given award number.  TODO: Seems a little more complex than just a straight lookup. |
| 0x10 | Exp (4) | Current Experience (max 4,294,967,296) |
| 0x11 | PartyPoison (1) | Checks party magical resistance to poison, as set by Prot. from Elements spell. |
| 0x12 | Condition (1) | Is under the given condition.  0=Cursed, 1=HeartBroken, 2=Weak, 3=Poisoned, 4=Diseased, 5=Insane, 6=InLove, 7=Drunk |
| 0x13 | CanCast (1) | Checks if the character class is capable of casting the given spell number, and then if the character has actually purchased that spell. See AllSpells for a list of spells, who can cast them, and their corresponding numbers |
| 0x14 | GameFlag (1) | Checks if the given game flag is set.  See Game_bit. |
| 0x15 | Item (1) | Searches inventories for given item number.  Returns true if found. |
| 0x16 | --- | UNUSED: This does nothing. Do Not use! |
| 0x17 | --- | UNUSED: This does nothing. Do Not use! |
| 0x18 | --- | UNUSED: This does nothing. Do Not use! |
| 0x19 | Minutes (2) | Minutes since midnight.  There is no "hour" variable (max 1,440) |
| 0x1A | --- | UNUSED: This does nothing. Do Not use! |
| 0x1B | --- | UNUSED: This does nothing. Do Not use! |
| 0x1C | --- | UNUSED: This does nothing. Do Not use! |
| 0x1D | --- | UNUSED: This does nothing. Do Not use! |
| 0x1E | --- | UNUSED: This does nothing. Do Not use! |
| 0x1F | --- | UNUSED: This does nothing. Do Not use! |
| 0x20 | --- | UNUSED: This does nothing. Do Not use! |
| 0x21 | --- | UNUSED: This does nothing. Do Not use! |
| 0x22 | Gold (4) | Current party gold (max 4,294,967,296) |
| 0x23 | Gems (2) | Current party gems (max 65,536) |
| 0x24 | --- | UNUSED: This does nothing. Do Not use! |
| 0x25 | MightTmp | Might Bonus (extra beyond base) |
| 0x26 | IntTmp | Intelligence Bonus (extra beyond base) |
| 0x27 | PersTmp | Personality Bonus (extra beyond base) |
| 0x28 | EndTmp | Endurance Bonus (extra beyond base) |
| 0x29 | SpdTmp | Speed Bonus (extra beyond base) |
| 0x2A | AccTmp | Accuracy Bonus (extra beyond base) |
| 0x2B | LuckTmp | Luck Bonus (extra beyond base) |
| 0x2C | YesNo (1) | If YesNo flag is 0, then it displays a Yes/No prompt.  Clicking Yes makes the comparison true.  If flag is 1, no prompt is displayed, but execution is paused until a key is pressed and comparison is automatically true. |
| 0x2D | MightBase | Might Base (before bonus) |
| 0x2E | IntBase | Intelligence Base (before bonus) |
| 0x2F | PersBase | Personality Base (before bonus) |
| 0x30 | EndBase | Endurance Base (before bonus) |
| 0x31 | SpdBase | Speed Bonus Base (before bonus) |
| 0x32 | AccBase | Accuracy Base (before bonus) |
| 0x33 | LuckBase | Luck Base (before bonus) |
| 0x34 | FireBase | Fire Resistance Base (before bonus) |
| 0x35 | ElecBase | Electricity Resistance Base (before bonus) |
| 0x36 | ColdBase | Cold Resistance Base (before bonus) |
| 0x37 | PsnBase | Poison Resistance Base (before bonus) |
| 0x38 | NrgBase | Energy Resistance Base (before bonus) |
| 0x39 | MgcBase | Magic Resistance Base (before bonus) |
| 0x3A | FireTmp | Fire Resistance Bonus (extra beyond base) |
| 0x3B | ElecTmp | Electricity Resistance Bonus (extra beyond base) |
| 0x3C | ColdTmp | Cold Resistance Bonus (extra beyond base) |
| 0x3D | PsnTmp | Poison Resistance Bonus (extra beyond base) |
| 0x3E | NrgTmp | Energy Resistance Bonus (extra beyond base) |
| 0x3F | MgcTmp | Magic Resistance Bonus (extra beyond base) |
| 0x40 | LevelBase | Base level (before bonuses) |
| 0x41 | Food | Party Food Amount |
| 0x42 | --- | UNUSED: This does nothing. Do Not use! |
| 0x43 | --- | UNUSED: This does nothing. Do Not use! |
| 0x44 | --- | UNUSED: This does nothing. Do Not use! |
| 0x45 | Levitate | Comparison against whether levitate spell is active or not. |
| 0x46 | Light | Comparison against the amount of light. |
| 0x47 | PartyFire (1) | Checks party magical resistance to fire, as set by Prot. from Elements spell. |
| 0x48 | PartyElec (1) | Checks party magical resistance to electricity, as set by Prot. from Elements spell. |
| 0x49 | PartyCold (1) | Checks party magical resistance to cold, as set by Prot. from Elements spell. |
| 0x4A | --- | UNUSED: This does nothing. Do Not use! |
| 0x4B | --- | UNUSED: This does nothing. Do Not use! |
| 0x4C | Day | Day of the current year (100 days per year) |
| 0x4D | ACTmp | Armor Class Bonuses (extra beyond base) |
| 0x4E | HPFull | Boolean compare: 1 if current HP is greater than or equal to max HP, 0 if below |
| 0x4F | WizardEye | Comparison against whether WizardEye spell is active or not. |
| 0x50 | --- | UNUSED: This does nothing. Do Not use! |
| 0x51 | SPFull | Boolean compare: 1 if current SP is greater than or equal to max SP, 0 if below |
| 0x52 | --- | UNUSED: This does nothing. Do Not use! |
| 0x53 | --- | UNUSED: This does nothing. Do Not use! |
| 0x54 | UNK | Comparison against a currently unknown byte |
| 0x55 | Year | Current game year since start. |
| 0x56 | Might | Total Might (base plus bonus) |
| 0x57 | Int | Total Intelligence (base plus bonus) |
| 0x58 | Pers | Total Personality (base plus bonus) |
| 0x59 | End | Total Endurance (base plus bonus) |
| 0x5A | Spd | Total Speed Bonus (base plus bonus) |
| 0x5B | Acc | Total Accuracy (base plus bonus) |
| 0x5C | Luck | Total Luck (base plus bonus) |
| 0x5D | Week | Current week of current year (day of year / 10) |
| 0x5E | WalkOnWater | Comparison against whether Walk On Water spell is active or not. |
| 0x5F | --- | UNUSED: This does nothing. Do Not use! |
| 0x60 | --- | UNUSED: This does nothing. Do Not use! |
| 0x61 | --- | UNUSED: This does nothing. Do Not use! |
| 0x62 | --- | UNUSED: This does nothing. Do Not use! |
| 0x63 | PartySkill | Boolean compare: 5=All Members have Crusader, 9=At least 2 have Mountaineer, 11=At least 2 have PathFinder, 14=All members have Swimmer. All others return true if at least one member of the party has the selected skill. |
| 0x64 | --- | UNUSED: This does nothing. Do Not use! |
| 0x65 | --- | UNUSED: This does nothing. Do Not use! |
| 0x66 | Thievery | Compares to the characters thievery skill |
| 0x67 | WorldFlag (1) | Checks if the given world flag is set.  See World_bit. |
| 0x68 | QuestFlag (1) | Checks if the given quest flag is set.  See Quest_bit. |
| 0x69 | MGCredit | Test number of Megacredits in the party. Only used by King's Engineer in Castle Burlock |
| 0x6A | --- | UNUSED: This does nothing. Do Not use! |
| 0x6B | InParty | Unknown use. Appears to be incomplete.  Uses the currently selected character, but does no sanity checks against it.  Also references code blocks as data blocks. Probably not safe to use.  DO NOT USE! |

Additional notes: In theory the UNUSED codes can be implemented in future ports.  Discussion may be required.

TODO: 0x54 reference two bytes of currently unknown use.

## 0x0B - MoveObj

**Arguments**

*byte - The object number as given on the map.
*byte x
*byte y

**Description**

```
Moves the map object to the given coordinates.  If no event exists at that position, it defaults to "Nothing Here".
```

**Notes**

```
This opcode is how chests move, and huts turn into fire.  The X/Y coordinates can be made to be outside the map as well, to make it appear the object has disappeared.
```

## 0x0C - TakeOrGive

**Arguments**

* byte - Take Type
* byte - Take Value (1, 2, or 4 bytes, depending on type, mostly 1)
* byte - Give Type
* byte - Give Value (1, 2, or 4 bytes, depending on type, mostly 1)

**Description**

Takes something from or gives something to someone, depending on given type and value.  Can simultaneously take and give something.

**Notes**

Gives to the most recently "selected" character.  Characters can be selected either with the WhoWill, SelRndChar, or SetChar command.  By default, the first character is selected.  If the selected character is set to 0 (via SetChar with a 0 argument) then all characters lose or get the specified thing.  ie, to give all characters 60000 experience requires two lines:
```
0f 00                     // select all characters
0c 00 00 10 60 ea 00 00   // give 60000 experience to selected character (ie, all)
```
If the take or give fails (ie, if inventory is full, or there is no such item) then the code simply exits the event, with no chance of recovery.

While 0x0C, 0x1C, 0x1D, and 0x26 opcodes all execute the TakeGive command, there are differences which may warrant each to have their own page.

#### TakeGive types

| Type | Arguments | Description |
|---|---|---|
| 0x00 | --- | UNUSED: This does nothing. Do Not use! |
| 0x01 | --- | UNUSED: This does nothing. Do Not use! |
| 0x02 | --- | UNUSED: This does nothing. Do Not use! |
| 0x03 | Sex | Take: UNUSED.  Give: Sets character sex (0=male, 1=female) |
| 0x04 | Race | Take: UNUSED.  Give: Sets character race (0=Human, 1=Elf, 2=Dwarf, 3=Gnome, 4=H-Orc) |
| 0x05 | Class | Take: UNUSED.  Give: Sets character class (0=Knight, 1=Paladin, 2=Archer, 3=Cleric, 4=Sorcerer, 5=Robber, 6=Ninja, 7=Barbarian, 8=Druid, 9=Ranger) |
| 0x06 | --- | UNUSED: This does nothing. Do Not use! |
| 0x07 | --- | UNUSED: This does nothing. Do Not use! |
| 0x08 | HP (1) | Does damage (TakeHP) or heals (GiveHP, no DMGType) |
| 0x09 | SP (1) | Add/subtract Spell Points |
| 0x0A | AC (1) | Set Armor Class.  Updates the ACTemp value.  Can be reduce below zero. |
| 0x0B | LevelTmp (1) | Level bonus (extra beyond base).  Can be reduced below zero. |
| 0x0C | Age (1) | Age Temp.  Can be reduced below zero |
| 0x0D | Skill (1) | Take/Give Skill. 0=Theivary, 1=ArmsMaster, 2=Astrologer, 3=BodyBuilder, 4=Cartographer, 5=Crusader, 6=DirectionSense, 7=Linguist, 8=Merchant, 9=Mountaineer, 10=Navigator, 11=PathFinder, 12=PrayerMaster, 13=Prestidigitator, 14=Swimmer, 15=Tracker, 16=SpotSecretDoors, 17=DangerSense |
| 0x0E | --- | UNUSED: This does nothing. Do Not use! |
| 0x0F | Award | Removes/grants an award.  Award 8 plays a spell FX when given.  TODO: Seems a little more complex than just a straight lookup. |
| 0x10 | Exp (4) | Add/subtract player experience (max 4,294,967,296).  Unsigned; will integer wrap. |
| 0x11 | PartyPoison (1) | Add/remove party poison resistance, as if set by Prot. from Elements spell. |
| 0x12 | Condition (1) | Remove/set the given condition.  0=Cursed, 1=HeartBroken, 2=Weak, 3=Poisoned, 4=Diseased, 5=Insane, 6=InLove, 7=Drunk, 8=Asleep, 9=Depressed, 10=Confused, 11=Paralyzed, 12=Unconscious, 13=Dead, 14=Stoned, 15=Eradicated.  Give Only: 16=Clear all conditions |
| 0x13 | Spell (1) | Checks if the character class is capable of casting the given spell number, and gives or removes that spell if possible. See AllSpells for a list of spells, who can cast them, and their corresponding numbers.  Will not grant a spell that cannot be cast by that class. |
| 0x14 | GameFlag (1) | Sets/unsets the given game flag.  See Game_bit. |
| 0x15 | Item (1) | Gives the specified basic item number, or searches inventories until it finds and removes one.  Halts further event execution if the item can not be given/removed because inventory is full or item does not exist. |
| 0x16 | --- | UNUSED: This does nothing. Do Not use! |
| 0x17 | --- | UNUSED: This does nothing. Do Not use! |
| 0x18 | --- | UNUSED: This does nothing. Do Not use! |
| 0x19 | Minutes (2) | Move forward or backward by the given number of minutes (Takes 2 bytes arguments, but only the first is used).  Handles day/night transitions properly.  Behaves opposite to expected.  Take advances clock, give reverses clock. |
| 0x1A | --- | UNUSED: This does nothing. Do Not use! |
| 0x1B | --- | UNUSED: This does nothing. Do Not use! |
| 0x1C | --- | UNUSED: This does nothing. Do Not use! |
| 0x1D | --- | UNUSED: This does nothing. Do Not use! |
| 0x1E | --- | UNUSED: This does nothing. Do Not use! |
| 0x1F | --- | UNUSED: This does nothing. Do Not use! |
| 0x20 | --- | UNUSED: This does nothing. Do Not use! |
| 0x21 | --- | UNUSED: This does nothing. Do Not use! |
| 0x22 | Gold (4) | Take/Give party gold (max 4,294,967,296).  Take fails if there's not enough gold, displaying a "not enough" message, aborting further event execution. |
| 0x23 | Gems (2) | Take/Give party gems (max 65,536).  Take fails if there's not enough gems, displaying a "not enough" message, aborting further event execution. |
| 0x24 | --- | UNUSED: This does nothing. Do Not use! |
| 0x25 | MightTmp | Might Bonus (extra beyond base).  Can be reduced below zero.  Tops out at 255. |
| 0x26 | IntTmp | Intelligence Bonus (extra beyond base).  Can be reduced below zero.  Tops out at 255. |
| 0x27 | PersTmp | Personality Bonus (extra beyond base).  Can be reduced below zero.  Tops out at 255. |
| 0x28 | EndTmp | Endurance Bonus (extra beyond base).  Can be reduced below zero.  Tops out at 255. |
| 0x29 | SpdTmp | Speed Bonus (extra beyond base).  Can be reduced below zero.  Tops out at 255. |
| 0x2A | AccTmp | Accuracy Bonus (extra beyond base).  Can be reduced below zero.  Tops out at 255. |
| 0x2B | LuckTmp | Luck Bonus (extra beyond base).  Can be reduced below zero.  Tops out at 255. |
| 0x2C | --- | UNUSED: This does nothing. Do Not use! |
| 0x2D | MightBase | Might Base (before bonus).  Tops out at 255. |
| 0x2E | IntBase | Intelligence Base (before bonus).  Tops out at 255. |
| 0x2F | PersBase | Personality Base (before bonus).  Tops out at 255. |
| 0x30 | EndBase | Endurance Base (before bonus).  Tops out at 255. |
| 0x31 | SpdBase | Speed Bonus Base (before bonus).  Tops out at 255. |
| 0x32 | AccBase | Accuracy Base (before bonus).  Tops out at 255. |
| 0x33 | LuckBase | Luck Base (before bonus).  Tops out at 255. |
| 0x34 | FireBase | Fire Resistance Base (before bonus).  Tops out at 255. |
| 0x35 | ElecBase | Electricity Resistance Base (before bonus).  Tops out at 255. |
| 0x36 | ColdBase | Cold Resistance Base (before bonus).  Tops out at 255. |
| 0x37 | PsnBase | Poison Resistance Base (before bonus).  Tops out at 255. |
| 0x38 | NrgBase | Energy Resistance Base (before bonus).  Tops out at 255. |
| 0x39 | MgcBase | Magic Resistance Base (before bonus).  Tops out at 255. |
| 0x3A | FireTmp | Fire Resistance Bonus (extra beyond base).  Tops out at 255. |
| 0x3B | ElecTmp | Electricity Resistance Bonus (extra beyond base).  Tops out at 255. |
| 0x3C | ColdTmp | Cold Resistance Bonus (extra beyond base).  Tops out at 255. |
| 0x3D | PsnTmp | Poison Resistance Bonus (extra beyond base).  Tops out at 255. |
| 0x3E | NrgTmp | Energy Resistance Bonus (extra beyond base).  Tops out at 255. |
| 0x3F | MgcTmp | Magic Resistance Bonus (extra beyond base).  Tops out at 255. |
| 0x40 | LevelBase | Base level (before bonuses).  Tops out at 255. |
| 0x41 | Food | Party Food Amount.  Take fails if there's not enough gold, displaying a "not enough" message, aborting further event execution. |
| 0x42 | --- | Take: UNUSED. Give: Unknown.  Seems to somehow create an item of some kind. |
| 0x43 | --- | UNUSED: This does nothing. Do Not use! |
| 0x44 | --- | UNUSED: This does nothing. Do Not use! |
| 0x45 | Levitate | Reduces/increase levitate count. |
| 0x46 | Light | Reduces/increase remaining light |
| 0x47 | PartyFire (1) | Add/remove party fire resistance, as if set by Prot. from Elements spell. |
| 0x48 | PartyElec (1) | Add/remove party electricity resistance, as if set by Prot. from Elements spell. |
| 0x49 | PartyCold (1) | Add/remove party cold resistance, as if set by Prot. from Elements spell. |
| 0x4A | DayOfSorcery | Add/remove levitate, light, walk on water spells, and party elemental resistances. |
| 0x4B | --- | UNUSED: This does nothing. Do Not use! |
| 0x4C | Day | Advance/subtract days.  Wraps through years, as well.  100 days per year. |
| 0x4D | ACTmp | Set Armor Class.  Updates the ACTemp value.  Can be reduce below zero. |
| 0x4E | xxx | Take: UNUSED.  Give: Set HP to HPMax.  Takes 1 byte argument, but does not use it. |
| 0x4F | WizardEye | Activate/deactivate Wizard Eye spell. |
| 0x50 | --- | UNUSED: This does nothing. Do Not use! |
| 0x51 | xxx | Take: UNUSED.  Give: Set SP to SPMax.  Takes 1 byte argument, but does not use it. |
| 0x52 | HP (1) | Take: UNUSED.  Give: GIVES damage (subtracts HP).  Behaves exactly like Take HP (0x08) above. |
| 0x53 | --- | UNUSED: This does nothing. Do Not use! |
| 0x54 | --- | UNUSED: This does nothing. Do Not use! |
| 0x55 | Year | Add/subtract from year.  Resets year flags as if a year had actually passed. |
| 0x56 | --- | UNUSED: This does nothing. Do Not use! |
| 0x57 | --- | UNUSED: This does nothing. Do Not use! |
| 0x58 | --- | UNUSED: This does nothing. Do Not use! |
| 0x59 | --- | UNUSED: This does nothing. Do Not use! |
| 0x5A | --- | UNUSED: This does nothing. Do Not use! |
| 0x5B | --- | UNUSED: This does nothing. Do Not use! |
| 0x5C | --- | UNUSED: This does nothing. Do Not use! |
| 0x5D | --- | UNUSED: This does nothing. Do Not use! |
| 0x5E | WalkOnWater | Activates/deactivates Walk On Water spell. |
| 0x5F | --- | UNUSED: This does nothing. Do Not use! |
| 0x60 | --- | UNUSED: This does nothing. Do Not use! |
| 0x61 | --- | UNUSED: This does nothing. Do Not use! |
| 0x62 | --- | UNUSED: This does nothing. Do Not use! |
| 0x63 | --- | UNUSED: This does nothing. Do Not use! |
| 0x64 | Gold(4) | Take: UNUSED.  Give: Gives a random amount of gold between 1 and value.  Takes 4 bytes argument, but only uses first two. (Max 65,536) |
| 0x65 | --- | UNUSED: This does nothing. Do Not use! |
| 0x66 | --- | UNUSED: This does nothing. Do Not use! |
| 0x67 | WorldFlag (1) | Sets/unsets world flag  See World_bit. |
| 0x68 | QuestFlag (1) | Checks if the given quest flag is set.  See Quest_bit. |
| 0x69 | --- | UNUSED: This does nothing. Do Not use! |
| 0x6A | --- | UNUSED: This does nothing. Do Not use! |
| 0x6B | InParty | Unknown use. Appears to be incomplete.  Uses the currently selected character, but does no sanity checks against it.  Also references code blocks as data blocks. Probably not safe to use.  DO NOT USE! |

TODO: How does 0x42 work?

## 0x0E - Remove

**Arguments**

none

**Description**

```
Removes the current event. This event can no longer be triggered by the user. This is usually paired with an AlterMap command to modify the map itself to remove the sprite that indicates that this tile contains an event; for example, the boats in the rivers on the Darkside that contain treasure, but disappear after you search them.```

## 0x0F - SetChar

**Arguments**

* byte - Selected character

**Description**

This opcode sets an internal "current character" flag used by several other operations.

If the argument is 0, the selected character is effectively "all".  If the selected character is 7, then a random character is chosen.  If greater than 7, then it asks in the exact same way as it would for WhoWill.

It seems that when a number n is set, the n first characters are concerned. See the holes in Yak Temple (X=5 Y=12 for example) when no levitate. The command is "SetChar 2" and the 2 first characters receive damages.

**Notes**

This does not, in fact, set the selected character.  If the argument is 1-6, it reads the byte, but then does nothing.  This may actually have been a bug, but since this command is only ever used to force events back to "select all" states (with an argument of 0) in the Xeen games, it probably went unnoticed.

## 0x10 - Spawn

**Arguments**

* byte monsterId
* byte x
* byte y
* byte (unknown) - unused

**Description**

```
Spawns (creates) the specified monster at the supplied location. This is used, for example, if you insult one of the NPCs and they spawn monsters to kill you. This is also used to set map flags used by IfMapFlag by spawning a "monster" outside the map, with either an X or Y position greater than 0x20.```

**Notes**

```
This effectively works more as a "move monster" than a spawn monster.  The monster of the given ID must already have been defined for the map.  It's stats are reset and it is placed at the X/Y coordinates, however.  A prime example is Gettlewaithe the Gremlin King in Castleview on the Darkside who spawns an army of gremlins and goblins around him when you talk to him.  Finally, the fourth byte appears totally unused.  It is always 0x00, and changing it has no effect.  It can be safely ignored.
```

## 0x11 - DoTownEvent

**Arguments**

* byte (enum) - the Town Event ID

**Description**

See the table below for details on how the engine handles this operation.

#### Town Event IDs
```
**Byte** **Value**
0x00 Bank
0x01 Blacksmith
0x02 Guild
0x03 Tavern
0x04 Temple
0x05 Trainer
0x06 Arena Event
0x07 Unknown
0x08 Reaper Event (Enter Tower)
0x09 Golem Event (Enter Dungeon)
0x0A Dwarf Event (Enter Dwarf's Mines in Clouds)
0x0B Sphinx Event
0x0C Pyramid
0x0D Dwarf Event (Enter Town in Darkside)
```

**Notes**

Implementation of each possible action is left up to the game engine, and may be implementation-specific. World of Xeen may handle the events differently than Swords of Xeen, but this has not been investigated yet.

## 0x12 - Exit

**Arguments**

none

**Description**

```
Simply exits the event.  No further lines will be executed.
```

## 0x13 - AlterMap

**Arguments**

*byte x
*byte y
*byte - Wall
*byte - Value

**Description**

```
Sets the wall value of the given X/Y coordinate of the current map.  See MAZExxx.DAT File Format for further details on the map structure.
```

**Notes**

This is only ever seen on indoor maps, which use wall values.  It should behave in a similar way on outdoor maps, using the 4 layers instead of 4 walls, but it's hard to be certain without an example.  In some single maze maps, the X and/or Y values are incremented by 16 as if they were the second half of a multi maze map.  See MAZE0054 (Witch Tower Level 4) for example.  This seems superfluous and easily ignorable.  Tests which set it back to the expected sub 16 value made no difference in the result.

## 0x15 - ConfirmWord

**Arguments**

* string (index into map text file) - acceptable word
* lineNumber - line number if an acceptable word is entered
* string (index into map text file) - second acceptable word, optional (zero means not used)
* string (index into map text file) - prompt to display to the player

**Description**

Displays the specified prompt to the player, and waits for them to enter a string. If this string matches the first acceptable word from the map text file, execution jumps to the supplied line number. If a second acceptable word index is provided (ie. is not zero) then this too is compared with the user input; a successful match also causes execution to jump to the supplied line number. If neither word matched, execution continues from the next line.

**Notes**

This opcode can have zero passed as the first acceptable word parameter, in which case it appears that it uses the result of a previous function. It also checks if the destination line number is zero, and this also affects the outcome of the functions. This needs to be further investigated.

Category:Event Opcodes

## 0x16 - Damage

**Arguments**

* 2xbyte - Amount
* byte - Type

**Description**

Does damage to the currently selected character(s) (as set by WhoWill, SetChar, or SelRndChar).

**Notes**

Displays the graphic and plays the sound effect for the respective damage type.  The elemental damage (fire, electricity, cold, and poison) can be negated by party resistance, and all but type 0x0 get a "saving throw" to avoid some (or all) of the damage.  The exact method of calculating the damage needs to be completed. 0x7 damage arguments are ignored, adds the condition Dead and sets hit points to 0, using SetChar = 0x0 and 0x7 Damage will result in a game over.

| Type | Description |
|---|---|
| 0x0 | Physical |
| 0x1 | Magic |
| 0x2 | Fire |
| 0x3 | Electricity |
| 0x4 | Cold |
| 0x5 | Poison |
| 0x6 | Energy |
| 0x7 | Dead |

TODO: Figure out the exact algorithm used for calculating damage.

## 0x17 - JumpRnd

**Arguments**

* byte - Max random value
* byte - Comparison value
* lineNumber - destination line number if comparison is true

**Description**

Generates a random number between 1 and the first argument (Max), and jumps to destination if it matches the second argument (Comparison)

**Notes**

For example, used in the Cupie doll games outside Xeen's castle to give a percentage chance of winning the doll.

## 0x18 - AlterEvent

**Arguments**

*byte - line number
*byte - replacement opcode

**Description**

```
Replaces the given line number of the current event with the replacement opcode.  If the opcode to be replaced is PlayCD then it also updates the line immediately following it.
```

**Notes**

```
Appears to be widely used to set various If conditions to a NOP opcode. ie
00: If                 09 00 04  // unconditional jump to line 4
01: Display            02  // display string 2, ie "Now you see this" 
02: If                 09 2c 01 03  // wait for a key press, then move to line 3
03: Exit                 // Exit event
04: Display            01  // display string 1, ie "See this first"
05: If                 09 2c 01 06  // wait for a key press, then move to line 6
06: AlterEvent         00 00  // make line 0 opcode 0
```
The first time this event is triggered, the If condition at line 0 will cause the event to skip to line 4, displaying "See this first" to the screen.  AlterEvent then changes the If condition to a NOP, so that the second time this event is triggered, it moves on to line 2, displaying "Now you see this".

The check for 0x3C was most likely a quick fix when CD voices was added to the game.  Rather than go through every event, and increment the line number to be updated if the line it was replacing was the newly inserted CD event (a time consuming task, easy to get wrong) they simply change both the newly inserted CD event, and the line which immediately follows which would be the correct event to change.  This is just a logical guess, and there may be more going on then is apparent.

## 0x19 - CallEvent

**Arguments**

*X-Coordinate (byte)
*Y-Coordinate (byte)
*Event line number to execute (byte)

**Description**

```
Calls the event at the specified location (X,Y), with the specified line number. This event is executed as if it
```
were part of the current event, after which execution resumes at the line following this command in the original event.

This is used, for example, when an identical set of actions is performed many times. If you have ten poison-spraying vents that each deal 100 poison damage to the party, you can create a dummy event at (40,40), and then have each of the ten vent events just call this dummy event to actually deal their damage when the party enters the square. If you later wish to change the amount of damage dealt, all you have to do is change the code at the called event at (40,40).

**Notes**

Hopefully any new IDE developed will support a more object-oriented way of scripting events, rendering this "gosub" equivalent command obsolete. The engine does not support more then 255 total lines (Original event + CallEvent) and using a CallEvent, Return then another CallEvent will cause the engine to have issues processing them, which will cause the event to stall or not complete

## 0x1A - Return

**Arguments**

* none

**Description**

Returns to the previous location in the stack following a CallEvent command.

**Notes**

Takes no arguments, has no intelligence.  Calling return from a function that wasn't started with CallEvent is undefined behavior.  Don't do it.  CallEvent opcodes can also stack, with each event calling another event before returning.  Just take care in unwinding the stack with your return calls.

## 0x1B - SetVar

**Arguments**

* byte - Variable ID
* byte - New Value
* byte - New Value [optional]
* byte - New Value [optional]
* byte - New Value [optional]

**Description**

Sets a game variable.  Variables can be character (race, sex, or temporary age) or party (gold, gems, or date/time) specific.  Most variables take a single byte, but some take two or four.  Note the chart below.  The variable is applied to the currently selected character (for example, by WhoWill or SetChar) or to all if selected character is 0 or 8.

**Notes**

For those arguments that take 2 or 4 values, watch the endianness.  ie, 0x10 0xA0 0x86 0x01 = 0x0186A0 Experience = 100'000 Experience.
Also, the set_nothings can probably be expanded and implemented in future games, if needed.

| Type | Arguments | Description |
|---|---|---|
| 0x00 | 1 | set_nothing |
| 0x01 | 1 | set_nothing |
| 0x02 | 1 | set_nothing |
| 0x03 | 1 | setSex |
| 0x04 | 1 | setRace |
| 0x05 | 1 | setClass |
| 0x06 | 1 | set_nothing |
| 0x07 | 1 | set_nothing |
| 0x08 | 1 | setCurHp |
| 0x09 | 1 | setCurSp |
| 0x0A | 1 | setAcTemp2 |
| 0x0B | 1 | setLevelTemp |
| 0x0C | 1 | setAgeTemp |
| 0x0D | 1 | set_nothing |
| 0x0E | 1 | set_nothing |
| 0x0F | 1 | set_nothing |
| 0x10 | 4 | setExperience |
| 0x11 | 1 | setEpPoison |
| 0x12 | 1 | *unknown* |
| 0x13 | 1 | set_nothing |
| 0x14 | 1 | set_nothing |
| 0x15 | 1 | set_nothing |
| 0x16 | 1 | set_nothing |
| 0x17 | 1 | set_nothing |
| 0x18 | 1 | set_nothing |
| 0x19 | 2 | setMinutes |
| 0x1A | 1 | set_nothing |
| 0x1B | 1 | set_nothing |
| 0x1C | 1 | set_nothing |
| 0x1D | 1 | set_nothing |
| 0x1E | 1 | set_nothing |
| 0x1F | 1 | set_nothing |
| 0x20 | 1 | set_nothing |
| 0x21 | 1 | set_nothing |
| 0x22 | 4 | setGold |
| 0x23 | 2 | setGems |
| 0x24 | 1 | set_nothing |
| 0x25 | 1 | setMgtTemp |
| 0x26 | 1 | setIntTemp |
| 0x27 | 1 | setPersTemp |
| 0x28 | 1 | setEndTemp |
| 0x29 | 1 | setSpdTemp |
| 0x2A | 1 | setAccTemp |
| 0x2B | 1 | setLuckTemp |
| 0x2C | 1 | set_nothing |
| 0x2D | 1 | setMgtBase |
| 0x2E | 1 | setIntBase |
| 0x2F | 1 | setPersBase |
| 0x30 | 1 | setEndBase |
| 0x31 | 1 | setSpdBase |
| 0x32 | 1 | setAccBase |
| 0x33 | 1 | setLuckBase |
| 0x34 | 1 | setChFireBase |
| 0x35 | 1 | setChElecBase |
| 0x36 | 1 | setChColdBase |
| 0x37 | 1 | setChPoisBase |
| 0x38 | 1 | setChEnrgBase |
| 0x39 | 1 | setChMagBase |
| 0x3A | 1 | setChFireTemp |
| 0x3B | 1 | setChElecTemp |
| 0x3C | 1 | setChColdTemp |
| 0x3D | 1 | setChPoisTemp |
| 0x3E | 1 | setChEnrgTemp |
| 0x3F | 1 | setChMagTemp |
| 0x40 | 1 | setLevelBase |
| 0x41 | 1 | setFood |
| 0x42 | 1 | set_nothing |
| 0x43 | 1 | set_nothing |
| 0x44 | 1 | set_nothing |
| 0x45 | 1 | setLevitate |
| 0x46 | 1 | setLight |
| 0x47 | 1 | setEpFire |
| 0x48 | 1 | setEpElec |
| 0x49 | 1 | setEpCold |
| 0x4A | 1 | setEpPoison |
| 0x4B | 1 | set_nothing |
| 0x4C | 1 | *unknown* |
| 0x4D | 1 | setAcTemp |
| 0x4E | 1 | set_nothing |
| 0x4F | 1 | setWizardEyeOn |
| 0x50 | 1 | set_nothing |
| 0x51 | 1 | set_nothing |
| 0x52 | 1 | set_nothing |
| 0x53 | 1 | *unknown* |
| 0x54 | 1 | setDirection |
| 0x55 | 1 | setYear |
| 0x56 | 1 | set_nothing |
| 0x57 | 1 | set_nothing |
| 0x58 | 1 | set_nothing |
| 0x59 | 1 | set_nothing |
| 0x5A | 1 | set_nothing |
| 0x5B | 1 | set_nothing |
| 0x5C | 1 | set_nothing |
| 0x5D | 1 | set_nothing |
| 0x5E | 1 | setWalkOnWater |

TODO: What do 0x12, 0x4C, 0x53 set?

## 0x1C - TakeOrGive

**Arguments**

* byte - Give Type
* byte - Compare Value (1, 2, or 4 bytes, depending on type, mostly 1)
* byte - Give Type
* byte - Give Value (1, 2, or 4 bytes, depending on type, mostly 1)

**Description**

Compares the value of the type in the first argument, and gives the thing in the second argument only if true.

**Notes**

Behaves similar to the 0x0C TakeOrGive except the "take" argument is replaced with the comparison.
For example
```
00: 1c 4d 00 4d 14 // if temp AC <= 0, give +20 AC
```
Checks temp armour (0x4D), and if it is 0, sets it to 0x14 (20).  Has the same effect as performing an If followed by a 0x0C TakeOrGive, and can be rewritten as
```
00: 09 4d 00 02    // if temp AC == 0, goto 2
01: 12             // else, exit
02: 0c 00 00 4d 14 // give +20 AC
```
This is only used for the well in Newcastle after it has been reconstructed.  The check is superfluous because it performs the If fix mentioned above, and then performs the comparison anyway.

While 0x0C, 0x1C, 0x1D, and 0x26 opcodes all execute the TakeGive command, there are differences which may warrant each to have their own page.

## 0x1D - TakeOrGive

**Arguments**

* byte - Take Type
* byte - Take Value (1, 2, or 4 bytes, depending on type, mostly 1)
* byte - Give Type
* byte - Give Value (1, 2, or 4 bytes, depending on type, mostly 1)

**Description**

Uses the arguments as an If to check character stats before giving something.

**Notes**

```

```
Gives to the most recently "selected" character.  Characters can be selected either with the WhoWill, SelRndChar, or SetChar command.  By default, the first character is selected.  If the selected character is set to 0 (via SetChar with a 0 argument) then all characters lose or get the specified thing.  ie, to give all characters 60000 experience requires two lines:
```
0f 00                     // select all characters
0c 00 00 10 60 ea 00 00   // give 60000 experience to selected character (ie, all)
```
It's not entirely obvious what is given.  More investigation would be required.

This is never actually used on any map anywhere in Xeen.  It was most likely never completed or tested, and simply replaced by a combined If and 0x0C TakeOrGive.  DO NOT USE!

While 0x0C, 0x1C, 0x1D, and 0x26 opcodes all execute the TakeGive command, there are differences which may warrant each to have their own page.

TODO: Do we want to do serious investigation, or make an executive decision and say "don't use this"?  Easier to not use.  Ever.

## 0x1E - CutsceneEndClouds

**Arguments**

none

**Description**

```
When this opcode is executed, the engine plays the Clouds of Xeen end-game animation.
```

## 0x20 - WhoWill

**Arguments**

*byte - "Who Will {what}"
*string (index into map text file) - Description text

**Description**

```
Displays the description text (lookup from the map text file), the "Who Will" prompt, waits for the player to select a character, and stores the selection.  If the escape key is pressed instead, the select is canceled, and the event exits.
```

**Notes**

```
The result of the selection is stored internally in the VAR_C variable.  This is used in other opcodes, such as TakeOrGive.  If there are less than 6 characters, it will wait until a valid character is selected.  If there is only one character in the party, it skips the prompt and just selects 1 automatically.
```

The same "Who Will" code is called by the Open Grate command, and the SetChar opcode, except the "description" is one of the Who Will options below (same as the Who Will {What}) instead of a map text.

#### Who Will

| Value | {What} |
|---|---|
| 0x00 | Search |
| 0x01 | Open |
| 0x02 | Drink |
| 0x03 | Mine |
| 0x04 | Touch |
| 0x05 | Read |
| 0x06 | Learn |
| 0x07 | Take |
| 0x08 | Bang |
| 0x09 | Steal |
| 0x0A | Bribe |
| 0x0B | Pay |
| 0x0C | Sit |
| 0x0D | Try |
| 0x0E | Turn |
| 0x0F | Bathe |
| 0x10 | Destroy |
| 0x11 | Pull |
| 0x12 | Descend |
| 0x13 | Toss A Coin |
| 0x14 | Pray |
| 0x15 | Join |
| 0x16 | Act |
| 0x17 | Play |
| 0x18 | Push |
| 0x19 | Rub |
| 0x1A | Pick |
| 0x1B | Eat |
| 0x1C | Sign |
| 0x1D | Close |
| 0x1E | Look |
| 0x1F | Try |

## 0x21 - RndDamage

**Arguments**

* byte - Type
* byte - Max

**Description**

Does a random amount of damage (from 1 to Max) to the currently selected character(s) (as set by WhoWill, SetChar, or SelRndChar).

**Notes**

Displays the graphic and plays the sound effect for the respective damage type.  The elemental damage (fire, elec, cold, and poison) can be negated by party resistance, and all but type 0x0 get a "saving throw" to avoid some (or all) of the damage.  The exact method of calculating the damage needs to be completed.

| Type | Description |
|---|---|
| 0x0 | Physical |
| 0x1 | Magic |
| 0x2 | Fire |
| 0x3 | Electricity |
| 0x4 | Cold |
| 0x5 | Poison |
| 0x6 | Energy |
| 0x7 | Dead |

TODO: Fill in the four remaining types, figure out the exact algorithm used for calculating damage.

## 0x22 - MoveWallObj

**Arguments**

*byte - The wall object number as given on the map.
*byte x
*byte y

**Description**

```
Moves the wall object to the given coordinates.  Doesn't change it's orientation.  Wall objects are only visible when viewed straight on, and were never intended to be anywhere but on squares directly facing walls.
```

**Notes**

```
Sample use is the gold in the dwarf mines.  The gold on the wall is actually a wall object immediately *in front* of the wall in question.  This opcode (combined with GiveExtended and AlterEvent) gives the illusion of the gold being mined by moving the wall object to position 128x128, well outside of the map.
```

## 0x23 - AlterCellFlag

**Arguments**

*byte x
*byte y
*byte - New Flag

**Description**

```
Sets the cell flag at the specified X/Y coordinate on the current map.
```

**Notes**

The only known usage of this opcode is in the Cave of Illusion Level 4 where pulling the plug causes all the water tiles (flag 0x0C) to become normal floor (0x00), and Volcano Cave Level 2 where hitting switches causes lava to become safe tiles.  Note also, it seems to behave slightly differently on outdoor maps than indoor, but since there are no outdoor maps that call this, it's difficult to experiment, and the point is moot anyway.

TODO: Decide how to handle this opcode on outdoor maps.  I've only experimented with Clouds data, so it might be relevant to Darkside.  If not, we can either try to work it out by creating our own dummy events, or just make an executive decision, ie, that it can't run on outdoor maps, or that it behaves exactly as indoor maps do.

## 0x24 - AlterHed

**Arguments**

*2xbyte - New value

**Description**

```
Sets the word value at the current X/Y location in the HED file in memory to the given two bytes
```

**Notes**

```
The only place in Xeen this is used is drinking out of the well in Vertigo.  The reason for this editing instruction is totally unknown, since the HED files are stored in the CC file, and not the SAV files, and therefore not remembered between loads and saves.  Even in the context of the well, it makes no sense.  Best guess suggests this was originally intended to serve a different purpose but was either never completed, or the original purpose of files changed and this instruction became useless.  This opcode can safely be ignored.
```

## 0x25 - DisplayStat

**Arguments**

* string (index into map text file) - String format to display

**Description**

Displays a text string which includes some stat of the currently selected character.

**Notes**

This opcode is never used in any map on either side of Xeen.  It appears to displays to the exact same window as DisplayBottomTwoLines.  The string appears to be a standard print format string which accepts and displays some currently unknown player stat.  THIS IS A VERY DANGEROUS OPCODE!  Unlike other opcodes, it has no checks to make sure the selected character is valid.  It doesn't really seem to serve any function.  DO NOT USE!

## 0x26 - TakeOrGive

**Arguments**

* byte - Give Type
* byte - Compare Value (1, 2, or 4 bytes, depending on type, mostly 1)
* byte - Give Type
* byte - Give Value (1, 2, or 4 bytes, depending on type, mostly 1)

**Description**

Compares the value of the type in the first argument, and gives the thing in the second argument only if compare value is less than the actual.

**Notes**

Behaves similar to the 0x1C TakeOrGive except the comparison is only true if the value is less than the actual, not greater than.

While 0x0C, 0x1C, 0x1D, and 0x26 opcodes all execute the TakeGive command, there are differences which may warrant each to have their own page.

**Do not use.** This is never used in Xeen, and is in fact implemented with bugs. It may be implemented in future ports and fixed, but don't count on it.

## 0x27 - SignTextSml

**Arguments**

* string (index into map text file)

**Description**

right
This operation draws the supplied string in the 3D game window. The text is drawn using the small font set, formatted and aligned so that it would appear on the back of a seat, and this opcode is only ever used in events directly in front of a seat (or similar) sprite. A typical example can be seen in the image to the right.

This is one of the few events that is automatically executed and does not need to be triggered by the player pressing the spacebar to interact with the event/object.

**Notes**

```
The functionality of this opcode is identical to SignTextLrg, except that it draws the text using the small font rather than the large font. See also DoorTextSml and DoorTextLrg.
```

## 0x28 - PlayEventVoc

**Arguments**

* byte - sound file array index

**Description**

An array of six VOC filenames are hard-coded into the game executable file. This function plays the VOC file at the specified index in this array.

```
0 - AHH.VOC
1 - WHERETO.VOC
2 - GULP.VOC
3 - NULL.VOC
4 - SCREAM.VOC
5 - LAFF1.VOC
```

**Notes**

This array is named **_eventSample** in the original game EXE file.

## 0x29 - DisplayBottom

**Arguments**

* string (index into map text file) - String to display

**Description**

```
Displays a large text message across the bottom of the screen.
```

**Notes**

```
Message runs across the entire bottom of the screen, overlaying the character portraits and arrow buttons.
```

## 0x2A - IfMapFlag

**Arguments**

* byte monsterId - Monster or flag number
* lineNumber - destination line number if comparison is true

**Description**

```
Checks if the given "flag" has been set, and jumps to the given line number if so.  A flag is technically a monster, and it is set by being moved outside of the map such that either the X or Y coordinate (or both) are greater than 0x20.  If the given ID is 0xFF, then it checks all monsters/flags and returns true iff all are set (outside the map).```

**Notes**

```
When a monster is killed, it's X and Y coordinates are both set to 0x80.  In this way, a check to see if all monsters have been killed on a given map can easily be performed.  With respect to flags, you can Spawn a "monster" either to 0x20 or greater (0x64 is usually used) to set the given flag, or to 0x00 to unset the flag.  Xeen can currently support up to 107 different monsters or flags on the map at once.
```

## 0x2B - SelRndChar

**Arguments**

none

**Description**

A random character from the party is selected and stored in a variable accessible by other scripting opcodes.

**Notes**

The character index is stored in *var_c*. TODO: Rename this variable.

## 0x2C - GiveEnchanted

**Arguments**

* byte - Item Type
* byte - Base Enchantment
* byte - Extra Enchantment
* byte - ???

**Description**

Gives an item with an enchantment.  Misc items have only a base enchantment, such as "of Cold Ray" or "of Antidote".  Equipable items use the base enchantment as their material, such as "wood" or "obsidian", while the extra enchantment is the actual bonus applied to it, such as "Golem Slaying".  Purpose of the fourth byte has not yet been determined.

**Notes**

Gives the item to the first character, if there's room.  If there's no room, then it moves on to the second, and so on, until inventory is full.  Unlike TakeOrGive, this does not appear to exit if the give failed because of no room, simply returns and continues operation.

If the item is a miscellaneous item (item type 0x3C and up, but less than 0x52) then it will be added with a random number from 1 to 8 charges, except for potions (item 46) and scrolls (item 47) which have a charge of 1.

TODO: What is the last byte for?  Also flesh out all the items and enchantments.  Possibly to their own page.

## 0x2D - ItemType

**Arguments**

* byte - The item type to be set

**Description**

Sets an internal "ItemType" variable which is used in other opcodes (such as GiveExtended) for giving specific item types.

**Notes**

The only known use of this command is in the clouds above Witch Tower, and is immediately followed by a GiveExtended command.  It sets the type of item to be given when picking up the offerings in the clouds.  These types may or may not be universal.  ie, the same "type" field may be used in other opcodes, such as GiveEnchanted.

| Type | Description |
|---|---|
| 0x00 | *Random* |
| 0x01 | Long Sword |
| 0x02 | Short Sword |
| 0x03 | Broad Sword |
| 0x04 | Scimitar |
| 0x05 | Cutlass |
| 0x06 | Sabre |
| 0x07 | Club |
| 0x08 | Hand Axe |
| 0x09 | Katana |
| 0x0A | Nunchakas |
| 0x0B | Wakazashi |
| 0x0C | Dagger |
| 0x0D | Mace |
| 0x0E | Flail |
| 0x0F | Cudgel |
| 0x10 | Maul |
| 0x11 | Spear |
| 0x12 | Bardiche |
| 0x13 | Glaive |
| 0x14 | Halberd |
| 0x15 | Pike |
| 0x16 | Flamberge |
| 0x17 | Trident |
| 0x18 | Staff |
| 0x19 | Hammer |
| 0x1A | Naginita |
| 0x1B | Battle Axe |
| 0x1C | Grand Axe |
| 0x1D | Great Axe |
| 0x1E | Short Bow |
| 0x1F | Long Bow |
| 0x20 | Crossbow |
| 0x21 | Sling |
| 0x22 | Xeen Slayer Sword (yes, it is its own type) |
| 0x23 | ?? blank ?? |
| 0x24 | Robes |
| 0x25 | Scale Armor |
| 0x26 | Ring Mail |
| 0x27 | Chain Mail |
| 0x28 | Splint Mail |
| 0x29 | Plate Mail |
| 0x2A | Plate Armor |
| 0x2B | Shield |
| 0x2C | Helm |
| 0x2D | Boots |
| 0x2E | Cloak |
| 0x2F | Cape |
| 0x30 | Gauntlets |
| 0x31 | ?? Blank ?? |
| 0x32 | Ring |
| 0x33 | Belt |
| 0x34 | Broach |
| 0x35 | Medal |
| 0x36 | Charm |
| 0x37 | Cameo |
| 0x38 | Scarab |
| 0x39 | Pendant |
| 0x3A | Necklace |
| 0x3B | Amulet |
| 0x3C | ?? Blank ?? |
| 0x3D | Rod |
| 0x3E | Jewel |
| 0x3F | Gem |
| 0x40 | Box |
| 0x41 | Orb |
| 0x42 | Horn |
| 0x43 | Coin |
| 0x44 | Wand |
| 0x45 | Whistle |
| 0x46 | Potion |
| 0x47 | Scroll |
| 0x48 | ?? Bogus ?? |
| 0x49 | ?? Bogus ?? |
| 0x4A | ?? Bogus ?? |
| 0x4B | ?? Bogus ?? |
| 0x4C | ?? Bogus ?? |
| 0x4D | ?? Bogus ?? |
| 0x4E | ?? Bogus ?? |
| 0x4F | ?? Bogus ?? |
| 0x50 | ?? Bogus ?? |
| 0x51 | ?? Bogus ?? |
| 0x52 | Deed To Newcastle |
| 0x53 | Crystal Key to Witch Tower |
| 0x54 | Skeleton Key to Darzog's Tower |
| 0x55 | Enchanted Key to Tower of High Magic |
| 0x56 | Jeweled Amulet of the Northern Sphinx |
| 0x57 | Stone of a Thousand Terrors |
| 0x58 | Golem Stone of Admittance |
| 0x59 | Yak Stone of Opening |
| 0x5A | Xeen's Scepter of Temporal Distortion |
| 0x5B | Alacorn of Falista |
| 0x5C | Elixir of Restoration |
| 0x5D | Wand of Faery Magic |
| 0x5E | Princess Roxanne's Tiara |
| 0x5F | Holy Book of Elvenkind |
| 0x60 | Scarab of Imaging |
| 0x61 | Crystals of Piezoelectricity |
| 0x62 | Scroll of Insight |
| 0x63 | Phirna Root |
| 0x64 | Orothin's Bone Whistle |
| 0x65 | Barok's Magic Pendant |
| 0x66 | Ligono's Missing Skull |
| 0x67 | Last Flower of Summer |
| 0x68 | Last Raindrop of Spring |
| 0x69 | Last Snowflake of Winter |
| 0x6A | Last Leaf of Autumn |
| 0x6B | Ever Hot Lava Rock |
| 0x6C | King's Mega Credit |
| 0x6D | Excavation Permit |
| 0x6E | Cupie Doll |
| 0x6F | Might Doll |
| 0x70 | Speed Doll |
| 0x71 | Endurance Doll |
| 0x72 | Accuracy Doll |
| 0x73 | Luck Doll |
| 0x74 | Widget |
| 0x75 | Pass to Castleview |
| 0x76 | Pass to Sandcaster |
| 0x77 | Pass to Lakeside |
| 0x78 | Pass to Necropolis |
| 0x79 | Pass to Olympus |
| 0x7A | Key to Great Western Tower |
| 0x7B | Key to Great Southern Tower |
| 0x7C | Key to Great Eastern Tower |
| 0x7D | Key to Great Northern Tower |
| 0x7E | Key to Ellinger's Tower |
| 0x7F | Key to Dragon Tower |
| 0x80 | Key to Darkstone Tower |
| 0x81 | Key to Temple of Bark |
| 0x82 | Key to Dungeon of Lost Souls |
| 0x83 | Key to Ancient Pyramid |
| 0x84 | Key to Dungeon of Death |
| 0x85 | Amulet of the Southern Sphinx |
| 0x86 | Dragon Pharaoh's Orb |
| 0x87 | Cube of Power |
| 0x88 | Chime of Opening |
| 0x89 | Gold ID Card |
| 0x8A | Silver ID Card |
| 0x8B | Vulture Repellent |
| 0x8C | Bridle |
| 0x8D | Enchanted Bridle |
| 0x8E | Treasure Map (Goto E1 x1, y11) |
| 0x8F | NOTUSED |
| 0x90 | Fake Map |
| 0x91 | Onyx Necklace |
| 0x92 | Dragon Egg |
| 0x93 | Tribble |
| 0x94 | Golden Pegasus Statuette |
| 0x95 | Golden Dragon Statuette |
| 0x96 | Golden Griffin Statuette |
| 0x97 | Chalice of Protection |
| 0x98 | Jewel of Ages |
| 0x99 | Songebird of Serenity |
| 0x9A | Sandro's Heart |
| 0x9B | Ector's Ring |
| 0x9C | Vespar's Emerald Handle |
| 0x9D | Queen Kalindra's Crown |
| 0x9E | Caleb's Magnifying Glass |
| 0x9F | Soul Box |
| 0xA0 | Soul Box with Corak Inside |
| 0xA1 | Ruby Rock |
| 0xA2 | Emerald Rock |
| 0xA3 | Sapphire Rock |
| 0xA4 | Diamond Rock |
| 0xA5 | Monga Melon |
| 0xA6 | Energy Disk |

NOTES: 
*the Blank items above give nothing and do nothing.
*The Xeen Slayer Sword is it's own type, and giving it in this manner (calling GiveExtended) actually allows it to be enchanted as well.
*The Bogus items give a Misc item with the name "Bogus", complete with enchantment (ie, "Bogus of Cure Poison").  They should not be used, unless redefined.
*Items from 0x52 onward are quest items.  They can be given in this way, but it won't actually give the quest item as expected.  Instead, it gives an enchanted misc item. ie, "Deed to Newcastle of Fiery Flail".  This is undefined behavior and may change in any future ports.  Don't do it!

TODO: Determine if these are specific or universal item types

## 0x2E - MakeNothingHere

**Arguments**

none

**Description**

```
Replaces the current event with a standard event which displays the message "Nothing here" in the bottom right corner when the player interacts with the current object/event/tile.
```

center

**Notes**

```
Literally, this steps through the event and replaces all opcodes with NOP codes.
```

## 0x0D / 0x2F - NoAction

**Arguments**

none

**Description**

This opcode doesn't perform any particular action, but still runs the normal sanity checks. For example, it checks if everyone in the party is dead, and if so, ends the game.

**Notes**

Still need to discover where this is used, and why. It probably complements an opcode or a set of opcodes which omit the sanity checks for some reason.

## 0x30 - ChooseNumeric

**Arguments**

* byte n - Number of options (1-9)
* byte[n] - (N-1) each a one byte line number corresponding to the option selected.

**Description**

Displays number buttons (up to 9) in the action button window (replacing the shoot, magic, rest, kick, etc...), waits for a user to select a valid choice, and jumps to the corresponding line number.  The last option always continues to the next line, even if N (or more than N) line numbers are given.  If fewer than N line numbers are given, then the options without matching line numbers simply continue to the next line.

**Notes**

Setting the number of options to 0 prevents the player from making any choice, effectively locking the game.  Setting the number of options to greater than 9 causes an overflow and crashes the game.  Don't do these things.  It's not nice.

## 0x31 - DisplayBottomTwoLines

**Arguments**

* string (index into map text file) - String to display
* string (index into map text file) - String to display

**Description**

This displays two strings, one above the other, at the bottom of the window, and waits for a key press.

**Notes**

Text window is across the entire bottom of the screen, overlaying portraits and arrow keys.

## 0x32 - DisplayLarge

**Arguments**

* string (index into map text file) - String to display

**Description**

Displays a potentially massive message to the user, filling the 3D window, possibly spanning several pages.  This string is not loaded from the standard AAZExxxx.TXT file, but from an alternate AAZE2xxx.TXT file, used only for this opcode.  The lookup and text rendering is performed in the exact same manner as Display0x01 and other such display codes.

**Notes**

This opcode is the only opcode to function on the alternate AAZE2xxx.TXT files.  Probably due to size constraints, an alternative file and opcode was needed for larger texts, but this is only an educated guess.  It's only known use is the exceptionally long story on the statue of the Dungeon of Death Level 1 which acts as the hints to the crossword puzzle, and even that was broken into two records.  This should be avoided if possible since it creates potential for back porting as it creates potential hash collisions (the hash for AAZE2121.TXT and AAZE2025.TXT is the same, for example) leading the engine to think two (or more) files are in fact the same file.  It is possible to use this opcode across multiple maps if the collisions are handled very carefully, though a better solution will no doubt exist in future ports.  This opcode will be supported for legacy purposes only.

## 0x33 - ExchObj

**Arguments**

*byte - The first object number as given on the map.
*byte - The second object number as given on the map.

**Description**

```
Exchanges two objects on the map.
```

**Notes**

```
This is effectively the same as two MoveObj opcodes: move first object to second objects location, and then second object to the first objects original location.  Uses include exchanging a closed barrel with an open barrel, or turning a tent into a fire.
```

## 0x34 - FallToMap

**Arguments**

* byte mapId - Destination Map ID
* byte x
* byte y
* byte - Fall damage

**Description**

Teleports the party to the specified destination and stops execution of the current script. An animation is played in which the player characters scream, and the 3D world screen slides upwards to reveal the new map location. While the new map is being loaded, "Oops..." appears instead of the standard "Please Wait" message. The specified amount of fall damage is also given to each character.

## 0x35 - DisplayMain

**Arguments**

* string (index into map text file) - String to display

**Description**

Display a message to the user in the main screen.

**Notes**

Message fills the 3D window.

## 0x36 - Goto

**Arguments**

* byte - c_surface?
* lineNumber - line number to jump to

**Description**

If the c_surface argument matches that of the map, then it jumps to the given line.

**Notes**

This opcode is not called anywhere in any map on either side of Xeen.  It seems highly superfluous, and shouldn't be used.  May not be included in future ports.

TODO: How is the c_surface variable set?  Note that we don't really care.

## 0x37 - ConfirmWord2

**Arguments**

* string (index into map text file) - acceptable word
* lineNumber - line number if an acceptable word is entered
* string (index into map text file) - acceptable word
* lineNumber - line number if an acceptable word is entered

**Description**

Waits for the player to enter a string.  If the string matches the first acceptable word, it jumps to the first given line number.  If the string matches the second acceptable word, it jumps to the second line number.  If neither string matches, execution continues to the next line.

**Notes**

Other than the second acceptable word jumping to a second line instead of the same line, and no prompt, this behaves exactly as ConfirmWord.

## 0x38 - GotoRandom

**Arguments**

* byte n - the number of supplied line numbers
* byte[n] - each a one byte line number

**Description**

```
This operation randomly jumps to one of any number of predefined line numbers in the current event. It is essentially a 'goto' operation which transfers execution a randomly-selected line from the array of supplied line numbers.
```

For example, if the supplied arguments were "03 05 09 1A", the first byte indicates that there are three possible jump locations and that the jump locations are lines 5, 9, and 28 (0x1A).

**Notes**

```
The theoretical maximum number of potential line numbers that could be supplied is approximately 250, however this is not an issue as the maximum number of lines that make up an event is limited to 255, due to the way the event data is stored.
```

## 0x39 - CutsceneEndDarkside

**Arguments**

none

**Description**

```
When this opcode is executed, the engine updates the **wonDarkside** value in the XEEN.CFG config file, so that the option to view the Darkside end-game cutscene now appears on the game's main menu. It sets the party location to map 29, x 2, y 2, then plays the Darskide end-game animation.
```

## 0x3A - CutsceneEndWorld

**Arguments**

none

**Description**

```
When this opcode is executed, the engine updates the **wonThird** value in the XEEN.CFG config file, so that the option to view the World of Xeen end-game cutscene now appears on the game's main menu. It then plays the World of Xeen end-game animation.
```

## 0x3B - FlipWorld

**Arguments**

* Byte - Side flag

**Description**

If flag is 0, load Clouds data.  If flag is 1, load Darkside data.

**Notes**

This does not actually move to the given side instantly, nor automatically.  Instead, it appears to set an internal flag (byte_618B2) which is later used in the loadMap (and other) function.  The flag may be checked at the end of event handling.  More investigation is required.

Code actually only accepts 0 or 1, and does nothing otherwise, but should be easy enough to extend to support multi-world games.

TODO: Once the flag is set, when is it acted upon?

## 0x3C - PlayCD

**Arguments**

* none

**Description**

Plays the specified CD track, starting at the specified start frame and ending at the specified end frame. This opcode is used to play the NPC speech in the World of Xeen CD edition.

**Notes**

This function is only supported in the World of Xeen CD edition. None of the other original games support this opcode.
