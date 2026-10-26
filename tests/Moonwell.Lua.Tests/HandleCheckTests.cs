using System;
using System.Collections.Generic;
using System.IO;
using CSharpLua.CoreSystem;

namespace Moonwell.Lua.Tests;

public class HandleCheckTests
{
    // CSharp.lua writes (lightning)data as System.cast(Moonwell.lightning, data), data as unit as System.as and data is unit
    // as System.is, which look for the class in the value's metatable. The game's handles are userdata with the game's own
    // metatable, so the check never passes there; analyzer rule MW0035 rejects these forms. The cases cannot show it: the fake
    // world's handles are CSharp.lua objects. Here a .NET object stands in for a handle, as NLua gives it to Lua as userdata
    // with NLua's own metatable. If the fork's CSharp.lua stops checking casts to handle types, this test fails: then casts
    // can leave MW0035.
    [Fact]
    public void A_game_handle_fails_every_check_for_its_type()
    {
        var lines = Run("""
            local handle = Handle
            print(type(handle))
            print(tostring(pcall(System.cast, Moonwell.lightning, handle)))
            print(tostring(System.is(handle, Moonwell.lightning)))
            print(tostring(System.as(handle, Moonwell.lightning)))
            print(tostring(System.is(Moonwell.lightning(), Moonwell.lightning)))
            """);

        Assert.Equal(new[] { "userdata", "false", "false", "nil", "true" }, lines);
    }

    private static List<string> Run(string code)
    {
        var lines = new List<string>();
        using var lua = new NLua.Lua();
        lua.RegisterFunction("print", lines, typeof(List<string>).GetMethod(nameof(List<string>.Add)));
        // The game's Lua has no debug library, and CSharp.lua's core takes other paths without it.
        lua.DoString("debug = nil");
        foreach (var file in CoreSystemProvider.GetCoreSystemFiles())
        {
            lua.DoString(File.ReadAllText(Path.Combine(AppContext.BaseDirectory, file)));
        }

        lua.DoString(File.ReadAllText(Path.Combine(LuaScript.RepoRoot, "src", "Moonwell.Natives", "Generated", "types.lua")));
        lua["Handle"] = new object();
        lua.DoString(code);
        return lines;
    }
}
