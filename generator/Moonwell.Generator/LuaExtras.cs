namespace Moonwell.Generator;

/// <summary>
/// Functions a map's Lua can call that common.j and blizzard.j do not declare. FourCC is part of the game's Lua;
/// UnitAlive is a common.ai native that was seen callable from a map (2026-09-29).
/// </summary>
public static class LuaExtras
{
    private static readonly string[] FromCommonAi = { "UnitAlive" };

    public static void AddTo(JassScripts scripts, string commonAi)
    {
        scripts.Functions.Add(new JassFunction("FourCC", new[] { new JassParameter("string", "id") }, "integer", "lua"));

        var ai = new JassScripts();
        Jass.ReadInto(ai, commonAi, "common.ai");
        foreach (var name in FromCommonAi)
        {
            scripts.Functions.Add(ai.Functions.Single(f => f.Name == name));
        }
    }
}
