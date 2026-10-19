using Moonwell.Generator;

namespace Moonwell.Generator.Tests;

public class NativeTagsTests
{
    private const string Common = """
        type agent extends handle
        type widget extends agent
        type unit extends widget
        type item extends widget
        type loadoutslot extends handle
        type player extends handle
        type location extends agent
        type framehandle extends handle
        type hashtable extends agent

        native GetLocalPlayer takes nothing returns player
        native GetCameraTargetPositionX takes nothing returns real
        native GetCameraTargetPositionLoc takes nothing returns location
        native GetLocalizedString takes string source returns string
        native CreateUnit takes player id, integer unitid, real x, real y, real face returns unit
        native KillUnit takes unit whichUnit returns nothing
        native UnitAddAbility takes unit whichUnit, integer abilityId returns boolean
        native GetUnitX takes unit whichUnit returns real
        native GetUnitLoc takes unit whichUnit returns location
        native GetRandomInt takes integer lowBound, integer highBound returns integer
        native DisplayTextToPlayer takes player toPlayer, real x, real y, string message returns nothing
        native SetCameraPosition takes real x, real y returns nothing
        native BlzFrameSetText takes framehandle frame, string text returns nothing
        native BlzGetFrameByName takes string name, integer createContext returns framehandle
        native LoadLocationHandle takes hashtable table, integer parentKey, integer childKey returns location
        native ConvertRace takes integer i returns race
        native Player takes integer number returns player
        native IsPlayerInForce takes player whichPlayer, force whichForce returns boolean
        native SomeNewNative takes nothing returns nothing
        native ForGroup takes group whichGroup, code callback returns nothing
        native ForForce takes force whichForce, code callback returns nothing
        native EnumItemsInRect takes rect r, boolexpr filter, code actionFunc returns nothing
        native ClearSelection takes nothing returns nothing
        native SelectUnit takes unit whichUnit, boolean flag returns nothing
        native GetEnumUnit takes nothing returns unit
        native BlzUnitIsAlive takes unit whichUnit returns boolean
        native BlzUnitItemInBagSlot takes unit whichUnit, integer itemSlot returns item
        native BlzUnitItemInEquipmentSlot takes unit whichUnit, loadoutslot itemSlot returns item
        native BlzUnitUnequipItemFromSlot takes unit whichUnit, loadoutslot slot returns item
        native BlzGetHUDScale takes nothing returns real
        native BlzSetHUDScale takes integer percentage returns nothing
        native BlzLockHUDScale takes nothing returns nothing
        native BlzUnlockHUDScale takes nothing returns nothing
        native BlzGetCameraAllowsHotkeyTargetLock takes nothing returns boolean
        native BlzSetCameraAllowsHotkeyTargetLock takes boolean allows returns nothing
        native BlzSetThematicMusicAbsoluteVolume takes integer volume returns nothing
        native BlzGetItemTooltip takes item whichItem returns string
        native BlzGetAbilityResearchTooltip takes integer abilCode, integer level returns string
        native GetAllyColorFilterState takes nothing returns integer
        native BlzFrameGetParent takes framehandle frame returns framehandle
        """;

    private const string Blizzard = """
        globals
            unit bj_lastCreatedUnit = null
        endglobals

        function DisplayTextToForce takes force toForce, string message returns nothing
            if (IsPlayerInForce(GetLocalPlayer(), toForce)) then
                // Use only local code (no net traffic) within this block to avoid desyncs.
                call DisplayTextToPlayer(GetLocalPlayer(), 0, 0, message)
            endif
        endfunction

        function CreateUnitSaveLast takes player id, integer unitid returns unit
            set bj_lastCreatedUnit = CreateUnit(id, unitid, 0, 0, 0)
            return bj_lastCreatedUnit
        endfunction

        function SetLocalCount takes nothing returns nothing
            local integer count = 0
            set count = count + 1
        endfunction

        function NoteBJ takes string s returns nothing
            call DisplayTextToPlayer(Player(0), 0, 0, GetLocalizedString(s))
        endfunction

        function CameraXBJ takes nothing returns real
            return GetCameraTargetPositionX()
        endfunction

        function KillLaterBJ takes unit u returns nothing
            call ForGroup(null, function KillUnit)
            call DisplayTextToPlayer(Player(0), 0, 0, "KillUnit(")
        endfunction

        function SelectGroupBJEnum takes nothing returns nothing
            call SelectUnit( GetEnumUnit(), true )
        endfunction

        function SelectGroupForPlayerBJ takes group g, player whichPlayer returns nothing
            if (GetLocalPlayer() == whichPlayer) then
                // Use only local code (no net traffic) within this block to avoid desyncs.
                call ClearSelection()
                call ForGroup( g, function SelectGroupBJEnum )
            endif
        endfunction

        function SelectAllBJ takes group g returns nothing
            call SelectGroupForPlayerBJ(g, Player(0))
        endfunction

        function KillOthersBJ takes player whichPlayer, unit u returns nothing
            if (GetLocalPlayer() == whichPlayer) then
                call DisplayTextToPlayer(whichPlayer, 0, 0, "kept")
            else
                call KillUnit(u)
            endif
            call KillUnit(u)
        endfunction

        function MakeOthersBJ takes player whichPlayer returns nothing
            if (GetLocalPlayer() == whichPlayer) then
                call DisplayTextToPlayer(whichPlayer, 0, 0, "kept")
            else
                call CreateUnit(whichPlayer, 0, 0, 0, 0)
            endif
        endfunction
        """;

    private static IReadOnlyDictionary<string, NativeTag> Tags() => NativeTags.Classify(Jass.Read(Common, Blizzard));

    [Theory]
    [InlineData("GetLocalPlayer", NativeTag.MachineLocal)]
    [InlineData("GetCameraTargetPositionX", NativeTag.MachineLocal)]
    [InlineData("GetCameraTargetPositionLoc", NativeTag.MachineLocal | NativeTag.CreatesHandle)]
    [InlineData("GetLocalizedString", NativeTag.MachineLocal)]
    [InlineData("CreateUnit", NativeTag.CreatesHandle)]
    [InlineData("KillUnit", NativeTag.ChangesState)]
    [InlineData("UnitAddAbility", NativeTag.ChangesState)]
    [InlineData("GetUnitX", NativeTag.None)]
    [InlineData("GetUnitLoc", NativeTag.CreatesHandle)]
    [InlineData("GetRandomInt", NativeTag.ChangesState)]
    [InlineData("DisplayTextToPlayer", NativeTag.None)]
    [InlineData("SetCameraPosition", NativeTag.None)]
    [InlineData("BlzFrameSetText", NativeTag.None)]
    [InlineData("BlzGetFrameByName", NativeTag.CreatesHandle)]
    [InlineData("LoadLocationHandle", NativeTag.None)]
    [InlineData("ConvertRace", NativeTag.None)]
    [InlineData("Player", NativeTag.None)]
    [InlineData("SomeNewNative", NativeTag.ChangesState)]
    [InlineData("ForGroup", NativeTag.ChangesState)]
    [InlineData("ForForce", NativeTag.ChangesState)]
    [InlineData("EnumItemsInRect", NativeTag.ChangesState)]
    [InlineData("ClearSelection", NativeTag.None)]
    [InlineData("SelectUnit", NativeTag.None)]
    [InlineData("BlzGetItemTooltip", NativeTag.MachineLocal)]
    [InlineData("BlzGetAbilityResearchTooltip", NativeTag.MachineLocal)]
    [InlineData("GetAllyColorFilterState", NativeTag.MachineLocal)]
    [InlineData("BlzFrameGetParent", NativeTag.MachineLocal | NativeTag.CreatesHandle)]
    [InlineData("BlzUnitIsAlive", NativeTag.None)]
    [InlineData("BlzUnitItemInBagSlot", NativeTag.None)]
    [InlineData("BlzUnitItemInEquipmentSlot", NativeTag.None)]
    [InlineData("BlzUnitUnequipItemFromSlot", NativeTag.ChangesState)]
    [InlineData("BlzGetHUDScale", NativeTag.MachineLocal)]
    [InlineData("BlzSetHUDScale", NativeTag.None)]
    [InlineData("BlzLockHUDScale", NativeTag.None)]
    [InlineData("BlzUnlockHUDScale", NativeTag.None)]
    [InlineData("BlzGetCameraAllowsHotkeyTargetLock", NativeTag.MachineLocal)]
    [InlineData("BlzSetCameraAllowsHotkeyTargetLock", NativeTag.None)]
    [InlineData("BlzSetThematicMusicAbsoluteVolume", NativeTag.None)]
    public void TagsNatives(string name, NativeTag expected)
    {
        Assert.Equal(expected, Tags()[name]);
    }

    [Theory]
    [InlineData("DisplayTextToForce", NativeTag.None)]
    [InlineData("CreateUnitSaveLast", NativeTag.CreatesHandle | NativeTag.ChangesState)]
    [InlineData("SetLocalCount", NativeTag.None)]
    [InlineData("NoteBJ", NativeTag.None)]
    [InlineData("CameraXBJ", NativeTag.MachineLocal)]
    [InlineData("KillLaterBJ", NativeTag.ChangesState)]
    [InlineData("SelectGroupForPlayerBJ", NativeTag.ChangesState | NativeTag.Desyncs)]
    [InlineData("SelectAllBJ", NativeTag.ChangesState | NativeTag.Desyncs)]
    [InlineData("KillOthersBJ", NativeTag.ChangesState)]
    [InlineData("MakeOthersBJ", NativeTag.CreatesHandle | NativeTag.Desyncs)]
    public void TagsBlizzardFunctionsByTheirBodies(string name, NativeTag expected)
    {
        Assert.Equal(expected, Tags()[name]);
    }

    [Fact]
    public void ReadsWhatABodyCallsOnce()
    {
        var display = Jass.Read(Common, Blizzard).Functions.Single(f => f.Name == "DisplayTextToForce");

        Assert.Equal(new[] { "IsPlayerInForce", "GetLocalPlayer", "DisplayTextToPlayer" }, display.Calls);
        Assert.Equal(new[] { "DisplayTextToPlayer", "GetLocalPlayer" }, display.LocalCalls);
        Assert.False(display.SetsGlobal);
    }

    [Fact]
    public void ReadsWhatABodyCallsInItsLocalPlayerBlocks()
    {
        var functions = Jass.Read(Common, Blizzard).Functions;

        Assert.Equal(new[] { "ClearSelection", "ForGroup", "SelectGroupBJEnum" },
            functions.Single(f => f.Name == "SelectGroupForPlayerBJ").LocalCalls);
        Assert.Equal(new[] { "DisplayTextToPlayer", "KillUnit" }, functions.Single(f => f.Name == "KillOthersBJ").LocalCalls);
        Assert.Empty(functions.Single(f => f.Name == "SelectAllBJ").LocalCalls);
    }

    [Fact]
    public void WritesTagsAsAttributes()
    {
        var natives = NativesWriter.WriteNatives(Jass.Read(Common, Blizzard), "test");

        Assert.Contains("        [CreatesHandle]\n        public static extern unit CreateUnit(", natives);
        Assert.Contains("        [MachineLocal, CreatesHandle]\n        public static extern location GetCameraTargetPositionLoc(", natives);
        Assert.Contains("\"GetUnitX({0})\"\n        public static extern float GetUnitX(", natives);
        Assert.Contains("        [ChangesState, Desyncs]\n        public static extern void SelectGroupForPlayerBJ(", natives);
    }
}
