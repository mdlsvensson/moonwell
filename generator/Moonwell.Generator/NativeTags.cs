using System.Text.RegularExpressions;

namespace Moonwell.Generator;

/// <summary>What calling a native or a blizzard.j function does to a multiplayer game (decision 13).</summary>
[Flags]
public enum NativeTag
{
    /// <summary>Safe anywhere: reads synced state, or changes only what one player sees or hears.</summary>
    None = 0,

    /// <summary>Makes a new handle (and so a handle id) on the machine that calls it.</summary>
    CreatesHandle = 1,

    /// <summary>Changes the game's state, the shared random generator included.</summary>
    ChangesState = 2,

    /// <summary>Gives a different answer on each machine.</summary>
    MachineLocal = 4,

    /// <summary>Changes the game inside its own <c>GetLocalPlayer()</c> block, so it desyncs wherever it is called.</summary>
    Desyncs = 8,
}

/// <summary>
/// Tags each native and blizzard.j function. Natives are tagged by the lists and name rules below; a native none of
/// them covers changes state, so a new patch's natives are rejected inside <c>Local.For</c> until they are listed.
/// A blizzard.j function takes the handle and state tags of what its body calls, and changes state when it sets a
/// global; it is machine-local when it returns a value and calls a machine-local native. It desyncs when, inside its
/// own <c>if</c> on <c>GetLocalPlayer()</c>, it creates a handle or calls a native known to desync there, or when it
/// calls a function that desyncs. Other state changes in Blizzard's local blocks (pausing for a dialog, setting bj_
/// globals, fog and occlusion) are left to Blizzard; map code still may not make them inside <c>Local.For</c>.
/// </summary>
public static partial class NativeTags
{
    /// <summary>Natives whose answer differs per machine: the local player, the camera, the mouse and keyboard,
    /// frames, sounds, localised names and tooltips, heights and what a player may have changed locally. jassdoc's
    /// <c>@async</c> natives are all here.</summary>
    private static readonly HashSet<string> MachineLocal = new(StringComparer.Ordinal)
    {
        "GetLocalPlayer", "GetLocalizedString", "GetLocalizedHotkey", "BlzGetLocale", "SkinManagerGetLocalPath",
        "GetObjectName", "GetUnitName", "GetItemName", "GetDestructableName", "GetHeroProperName",
        "BlzGetItemDescription", "BlzGetItemTooltip", "BlzGetItemExtendedTooltip", "BlzGetAbilityTooltip",
        "BlzGetAbilityActivatedTooltip", "BlzGetAbilityExtendedTooltip", "BlzGetAbilityActivatedExtendedTooltip",
        "BlzGetAbilityResearchTooltip", "BlzGetAbilityResearchExtendedTooltip",
        "GetAllyColorFilterState", "GetCreepCampFilterState",
        "GetLocationZ", "BlzGetLocalUnitZ", "BlzGetUnitZ",
        "GetCameraBoundMinX", "GetCameraBoundMinY", "GetCameraBoundMaxX", "GetCameraBoundMaxY", "GetCameraField",
        "GetCameraMargin", "GetCameraTargetPositionX", "GetCameraTargetPositionY", "GetCameraTargetPositionZ",
        "GetCameraTargetPositionLoc", "GetCameraEyePositionX", "GetCameraEyePositionY", "GetCameraEyePositionZ",
        "GetCameraEyePositionLoc", "GetCameraFieldControlledByInput", "BlzCameraGetCameraType",
        "BlzIsMetaKeyPressed", "BlzIsKeyPressed", "BlzIsMouseButtonPressed", "BlzGetMouseScreenPosX",
        "BlzGetMouseScreenPosY", "BlzGetMouseFocusUnit", "BlzGetLocalClientWidth", "BlzGetLocalClientHeight",
        "BlzIsLocalClientActive", "BlzIsSelectionEnabled", "BlzIsSelectionCircleEnabled",
        "BlzFrameIsVisible", "BlzFrameGetName", "BlzFrameGetText", "BlzFrameGetTextSizeLimit", "BlzFrameGetEnable",
        "BlzFrameGetAlpha", "BlzFrameGetValue", "BlzFrameGetHeight", "BlzFrameGetWidth", "BlzFrameGetChildrenCount",
        "BlzFrameGetParent", "BlzFrameGetChild",
        "BlzFrameToPixelX", "BlzFrameToPixelY",
        "GetSoundDuration", "GetSoundFileDuration", "GetSoundIsPlaying", "GetSoundIsLoading",
        "IsCineFilterDisplayed", "IsMultiboardDisplayed", "IsMultiboardMinimized", "MultiboardGetTitleText",
        "IsLeaderboardDisplayed", "LeaderboardGetLabelText", "IsTimerDialogDisplayed",
        "BlzGetLocalSpecialEffectX", "BlzGetLocalSpecialEffectY", "BlzGetLocalSpecialEffectZ",
        "BlzGetSpecialEffectScale", "GetLightningColorA", "GetLightningColorR", "GetLightningColorG",
        "GetLightningColorB",
    };

    /// <summary>Natives that change only what the local player sees or hears, so they may run on one machine.</summary>
    private static readonly HashSet<string> LocalSafe = new(StringComparer.Ordinal)
    {
        "DisplayTextToPlayer", "DisplayTimedTextToPlayer", "DisplayTimedTextFromPlayer", "ClearTextMessages",
        "BlzDisplayChatMessage",
        "SetCameraPosition", "SetCameraQuickPosition", "SetCameraBounds", "StopCamera", "ResetToGameCamera",
        "SetCinematicCamera", "SetCameraRotateMode", "SetCameraField", "AdjustCameraField",
        "SetCameraFieldControlledByInput", "SetCameraTargetController", "SetCameraOrientController",
        "BlzCameraSetCameraType", "CameraSetupApply", "CameraSetupApplyWithZ", "CameraSetupApplyForceDuration",
        "CameraSetupApplyForceDurationWithZ", "BlzCameraSetupApplyForceDurationSmooth",
        "DisplayCineFilter", "SetCinematicScene", "EndCinematicScene", "ForceCinematicSubtitles",
        "BlzHideCinematicPanels",
        "StartSound", "StartSoundEx", "StopSound", "SetSoundVolume", "SetSoundPitch", "SetSoundPlayPosition",
        "SetSoundPosition", "SetSoundVelocity", "SetSoundDistances", "SetSoundDistanceCutoff", "SetSoundConeAngles",
        "SetSoundConeOrientation", "PlayMusic", "PlayMusicEx", "StopMusic", "ResumeMusic", "PlayThematicMusic",
        "PlayThematicMusicEx", "EndThematicMusic", "SetMusicVolume", "SetMusicPlayPosition",
        "SetThematicMusicVolume", "SetThematicMusicPlayPosition", "VolumeGroupSetVolume", "VolumeGroupReset",
        "SetMapMusic", "ClearMapMusic",
        "ShowInterface", "EnableUserControl", "EnableUserUI", "EnableDragSelect", "EnablePreSelect", "EnableSelect",
        "BlzEnableSelections", "BlzHideOriginFrames", "BlzEnableUIAutoPosition", "BlzFrameClearAllPoints",
        "BlzFrameAddText", "BlzFrameCageMouse", "BlzTextAreaFrameSetAutoScroll", "BlzEnableCursor",
        "BlzSetMousePos", "BlzChangeMinimapTerrainTex", "SetPortraitLight", "PingMinimap", "PingMinimapEx",
        "FlashQuestDialogButton", "SelectUnit", "ClearSelection",
        "ShowImage", "ShowUbersplat", "SetTerrainFog", "SetTerrainFogEx", "ResetTerrainFog",
        "MultiboardDisplay", "MultiboardMinimize", "LeaderboardDisplay", "TimerDialogDisplay",
        "Preload", "PreloadGenClear", "PreloadGenStart", "PreloadGenEnd",
    };

    /// <summary>Natives that desync when run in local code, whatever their callback does: ForGroup and ForForce by
    /// Hive thread 317486 (Insanity_AI on SelectGroupForPlayerBJ, which desyncs since 1.32), the Enum natives
    /// inferred, since they run a callback the same way. They change state.</summary>
    private static readonly HashSet<string> DesyncInLocal = new(StringComparer.Ordinal)
    {
        "ForGroup", "ForForce", "EnumDestructablesInRect", "EnumItemsInRect",
    };

    /// <summary>Name prefixes of local-safe natives: frame, text-tag, filter and camera setters, effect looks.</summary>
    private static readonly string[] LocalSafePrefixes =
    {
        "BlzFrameSet", "SetTextTag", "SetCineFilter", "CameraSet", "PanCamera", "BlzSetSpecialEffect",
        "BlzSetTerrainFog",
    };

    /// <summary>Natives that read or compute without a Get-like name.</summary>
    private static readonly HashSet<string> Readers = new(StringComparer.Ordinal)
    {
        "Player", "FourCC", "OrderId", "OrderId2String", "UnitId", "UnitId2String", "AbilityId", "AbilityId2String",
        "StringHash", "StringLength", "SubString", "StringCase", "S2I", "I2S", "R2S", "R2SW", "S2R", "I2R", "R2I",
        "Deg2Rad", "Rad2Deg", "Sin", "Cos", "Tan", "Asin", "Acos", "Atan", "Atan2", "SquareRoot", "Pow",
        "BlzBitAnd", "BlzBitOr", "BlzBitXor", "BlzConvertColor", "BlzFourCC2S", "BlzS2FourCC", "ParseTags",
        "UnitAlive", "UnitItemInSlot", "FirstOfGroup", "BlzGroupUnitAt", "BlzGroupGetSize", "UnitInventorySize",
        "RectContainsCoords", "TimerGetElapsed", "TimerGetRemaining", "TimerGetTimeout", "UnitItemInBagSlot",
        "UnitItemInEquipmentSlot", "VersionGet", "PlayerGetLeaderboard",
    };

    /// <summary>Natives that change state and return a handle that already existed.</summary>
    private static readonly HashSet<string> ReturnExisting = new(StringComparer.Ordinal)
    {
        "UnitRemoveItemFromSlot", "UnitUnequipItemFromSlot",
    };

    /// <summary>Natives that return a handle that did not exist before the call.</summary>
    private static readonly HashSet<string> NewHandles = new(StringComparer.Ordinal)
    {
        // The first time a frame is looked up it gets a handle id (Tasyen, game-facts.md).
        "BlzGetFrameByName", "BlzGetOriginFrame", "BlzFrameGetParent", "BlzFrameGetChild",
        "GetWorldBounds", "MultiboardGetItem",
    };

    /// <summary>Name prefixes that change state, checked before the Get-like words.</summary>
    private static readonly string[] StatePrefixes =
    {
        "Set", "Add", "Remove", "Destroy", "Kill", "Issue", "Create", "Save", "Store", "Flush", "Enable", "Disable",
        "Pause", "Unpause", "Start", "Stop", "Trigger", "Group", "Force", "Suspend", "Revive", "Select", "Reset",
        "Sync", "Execute", "Move", "Show", "Hide", "Clear", "Display", "Play", "Unit", "Blz",
    };

    public static IReadOnlyDictionary<string, NativeTag> Classify(JassScripts scripts)
    {
        var handles = new HashSet<string>(scripts.Types.Select(t => t.Name), StringComparer.Ordinal) { "handle" };
        var tags = new Dictionary<string, NativeTag>(StringComparer.Ordinal);
        foreach (var function in scripts.Functions)
        {
            tags[function.Name] = function.Source == "blizzard.j"
                ? FromBody(function, tags)
                : Native(function, handles);
        }

        return tags;
    }

    public static NativeTag Native(JassFunction function, IReadOnlySet<string> handles)
    {
        var name = function.Name;
        var newHandle = (function.Returns == "location" && !IsHashtableLoad(function)) || NewHandles.Contains(name);
        if (MachineLocal.Contains(name))
        {
            return NativeTag.MachineLocal | (newHandle ? NativeTag.CreatesHandle : NativeTag.None);
        }

        if (newHandle)
        {
            return NativeTag.CreatesHandle;
        }

        if (LocalSafe.Contains(name) || LocalSafePrefixes.Any(p => name.StartsWith(p, StringComparison.Ordinal)))
        {
            return NativeTag.None;
        }

        if (ReturnExisting.Contains(name))
        {
            return NativeTag.ChangesState;
        }

        if (name.StartsWith("GetRandom", StringComparison.Ordinal))
        {
            return NativeTag.ChangesState;
        }

        if (Readers.Contains(name) || name.StartsWith("Convert", StringComparison.Ordinal) || IsGetter(function))
        {
            return NativeTag.None;
        }

        var state = !StatePrefixes.Any(p => name.StartsWith(p, StringComparison.Ordinal))
            && function.Returns != "nothing" && GetterWord().IsMatch(name)
            ? NativeTag.None
            : NativeTag.ChangesState;
        if (state == NativeTag.None)
        {
            return NativeTag.None;
        }

        return handles.Contains(function.Returns) ? NativeTag.CreatesHandle : NativeTag.ChangesState;
    }

    private static bool IsGetter(JassFunction function)
    {
        if (GetterPrefix().IsMatch(function.Name))
        {
            return true;
        }

        return IsHashtableLoad(function);
    }

    /// <summary>LoadInteger and its kind read a hashtable; LoadGame does not.</summary>
    private static bool IsHashtableLoad(JassFunction function) =>
        function.Name.StartsWith("Load", StringComparison.Ordinal)
        && function.Parameters.Count > 0 && function.Parameters[0].Type == "hashtable";

    private static NativeTag FromBody(JassFunction function, IReadOnlyDictionary<string, NativeTag> known)
    {
        var tag = function.SetsGlobal ? NativeTag.ChangesState : NativeTag.None;
        foreach (var call in function.LocalCalls)
        {
            if (DesyncInLocal.Contains(call)
                || (known.TryGetValue(call, out var callee) && callee.HasFlag(NativeTag.CreatesHandle)))
            {
                tag |= NativeTag.Desyncs;
            }
        }

        foreach (var call in function.Calls)
        {
            if (!known.TryGetValue(call, out var callee))
            {
                continue;
            }

            tag |= callee & (NativeTag.CreatesHandle | NativeTag.ChangesState | NativeTag.Desyncs);

            // A function that returns nothing keeps what it read (comparing the local player is how it keeps its
            // work local, and localised strings go to the screen); one that returns a value may return it.
            if (function.Returns != "nothing")
            {
                tag |= callee & NativeTag.MachineLocal;
            }
        }

        return tag;
    }

    [GeneratedRegex(@"^(Blz)?(Get|Is|Has|Have)[A-Z0-9]")]
    private static partial Regex GetterPrefix();

    [GeneratedRegex(@"(Get|Is|Has|Have|Contains|Count|Size)[A-Z]|(Count|Size)$")]
    private static partial Regex GetterWord();
}
