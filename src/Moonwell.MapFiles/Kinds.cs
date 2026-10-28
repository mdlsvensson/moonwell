using Moonwell.GameData;

namespace Moonwell.MapFiles;

/// <summary>Facts about each of the seven object <see cref="Kind"/>s: their files, names and whether they have levels.</summary>
public static class Kinds
{
    /// <summary>The seven kinds, in the order of the <see cref="Kind"/> enum.</summary>
    public static IReadOnlyList<Kind> All { get; } = Enum.GetValues<Kind>();

    /// <summary>The kind's file extension without the dot: <c>w3u</c> for units.</summary>
    public static string Extension(Kind kind) => kind switch
    {
        Kind.Unit => "w3u",
        Kind.Item => "w3t",
        Kind.Destructable => "w3b",
        Kind.Doodad => "w3d",
        Kind.Ability => "w3a",
        Kind.Buff => "w3h",
        Kind.Upgrade => "w3q",
        _ => throw new ArgumentOutOfRangeException(nameof(kind), kind, null),
    };

    /// <summary>The kind's name in messages: <c>unit</c>, <c>item</c>, <c>destructable</c> and so on.</summary>
    public static string Key(Kind kind) => kind switch
    {
        Kind.Unit => "unit",
        Kind.Item => "item",
        Kind.Destructable => "destructable",
        Kind.Doodad => "doodad",
        Kind.Ability => "ability",
        Kind.Buff => "buff",
        Kind.Upgrade => "upgrade",
        _ => throw new ArgumentOutOfRangeException(nameof(kind), kind, null),
    };

    /// <summary>The kind with this <see cref="Key"/>, or null when no kind has it.</summary>
    public static Kind? FromKey(string key)
    {
        foreach (var kind in All)
        {
            if (Key(kind) == key)
            {
                return kind;
            }
        }

        return null;
    }

    /// <summary>
    /// Whether the kind's modifications carry a level (a variation for doodads) and a data pointer: doodads, abilities
    /// and upgrades.
    /// </summary>
    public static bool HasLevels(Kind kind) => kind is Kind.Doodad or Kind.Ability or Kind.Upgrade;

    /// <summary>The name of the kind's main file, such as <c>war3map.w3u</c>.</summary>
    public static string MainName(Kind kind) => "war3map." + Extension(kind);

    /// <summary>The name of the kind's skin file, such as <c>war3mapSkin.w3u</c>.</summary>
    public static string SkinName(Kind kind) => "war3mapSkin." + Extension(kind);
}
