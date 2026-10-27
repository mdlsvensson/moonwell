using System;
using System.Collections.Generic;
using System.IO;
using CSharpLua.CoreSystem;

namespace Moonwell.Lua.Tests;

public class NativesLuaTests
{
    // The Lua core's Natives.lua puts Lua's math functions in place of some of the game's natives, and Natives.Deg2Rad and
    // Rad2Deg are written as calls to those globals. CoreSystem up to 2.0.3-moonwell.4 had the two swapped (C41).
    [Fact]
    public void Deg2Rad_and_Rad2Deg_convert_the_right_way()
    {
        var lines = Run("""
            print(string.format('%.4f', Deg2Rad(180)))
            print(string.format('%.4f', Rad2Deg(math.pi)))
            """);

        Assert.Equal(new[] { "3.1416", "180.0000" }, lines);
    }

    private static List<string> Run(string code)
    {
        var lines = new List<string>();
        using var lua = new NLua.Lua();
        lua.RegisterFunction("print", lines, typeof(List<string>).GetMethod(nameof(List<string>.Add)));
        lua.DoString("debug = nil");
        foreach (var file in CoreSystemProvider.GetCoreSystemFiles())
        {
            lua.DoString(File.ReadAllText(Path.Combine(AppContext.BaseDirectory, file)));
        }

        lua.DoString(code);
        return lines;
    }
}
