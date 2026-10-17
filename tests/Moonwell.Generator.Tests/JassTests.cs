using Moonwell.Generator;

namespace Moonwell.Generator.Tests;

public class JassTests
{
    private const string Common = """
        type agent extends handle // reference counted
        type widget extends agent
        type unit extends widget
        type event extends agent

        globals
            constant integer    JASS_MAX_ARRAY_SIZE = 32768
            constant real       bj_PI = 3.14159
            constant string     NAME = "a // b"
            constant playerunitevent EVENT_PLAYER_UNIT_DEATH = ConvertPlayerUnitEvent(20)
            integer             counter = 0
            unit array          units
        endglobals

        native TimerStart takes timer whichTimer, real timeout, boolean periodic, code handlerFunc returns nothing
        constant native GetTriggerUnit takes nothing returns unit
        native Condition takes code func returns conditionfunc
        native TriggerRegisterTimerEvent takes trigger whichTrigger, real timeout, boolean periodic returns event
        """;

    private const string Blizzard = """
        globals
            force array bj_FORCE_PLAYER
        endglobals

        function RMinBJ takes real a, real b returns real
            if (a < b) then
                return a
            endif
            return b
        endfunction
        """;

    [Fact]
    public void ReadsTypesFunctionsAndGlobalsFromBothFiles()
    {
        var scripts = Jass.Read(Common, Blizzard);

        Assert.Equal(new[] { "agent", "widget", "unit", "event" }, scripts.Types.Select(t => t.Name));
        Assert.Equal(new[] { "TimerStart", "GetTriggerUnit", "Condition", "TriggerRegisterTimerEvent", "RMinBJ" },
            scripts.Functions.Select(f => f.Name));
        Assert.Equal("blizzard.j", scripts.Functions.Last().Source);
        Assert.Equal(7, scripts.Globals.Count);
        Assert.Equal("\"a // b\"", scripts.Globals.Single(g => g.Name == "NAME").Value);
        Assert.True(scripts.Globals.Single(g => g.Name == "units").IsArray);
        Assert.False(scripts.Globals.Single(g => g.Name == "counter").IsConstant);
    }

    [Fact]
    public void ReadsParameters()
    {
        var timerStart = Jass.Read(Common, Blizzard).Functions.First();

        Assert.Equal(new[] { "timer", "real", "boolean", "code" }, timerStart.Parameters.Select(p => p.Type));
        Assert.Equal("handlerFunc", timerStart.Parameters[3].Name);
        Assert.Equal("nothing", timerStart.Returns);
    }

    [Theory]
    [InlineData("32768", 32768)]
    [InlineData("-1", -1)]
    [InlineData("0x7FFFFFFF", int.MaxValue)]
    [InlineData("$FF", 255)]
    [InlineData("'hfoo'", 0x68666F6F)]
    [InlineData("'A'", 65)]
    public void ReadsIntegerLiterals(string text, int expected)
    {
        Assert.Equal(expected, NativesWriter.IntegerLiteral(text));
    }

    [Theory]
    [InlineData("ConvertRace(1)")]
    [InlineData("bj_PI * 2")]
    public void LeavesExpressionsUnread(string text)
    {
        Assert.Null(NativesWriter.IntegerLiteral(text));
    }

    [Fact]
    public void WritesConstantsVariablesArraysAndHandleConstants()
    {
        var natives = NativesWriter.WriteNatives(Jass.Read(Common, Blizzard), "test");

        Assert.Contains("public const int JASS_MAX_ARRAY_SIZE = 32768;", natives);
        Assert.Contains("public const float bj_PI = 3.14159f;", natives);
        Assert.Contains("public static extern playerunitevent EVENT_PLAYER_UNIT_DEATH { get; }", natives);
        Assert.Contains("/// @CSharpLua.Set = \"counter = {0}\"", natives);
        Assert.Contains("public static extern int counter { get; set; }", natives);
        Assert.Contains("/// @CSharpLua.Template = \"bj_FORCE_PLAYER[{0}]\"", natives);
        Assert.Contains("public static extern force bj_FORCE_PLAYER(int index);", natives);
    }

    [Fact]
    public void WritesFunctionsWithTemplatesCallbacksAndEscapedNames()
    {
        var natives = NativesWriter.WriteNatives(Jass.Read(Common, Blizzard), "test");

        Assert.Contains("/// @CSharpLua.Template = \"TimerStart({0}, {1}, {2}, {3})\"", natives);
        Assert.Contains("public static extern void TimerStart(timer whichTimer, float timeout, bool periodic, Action handlerFunc);", natives);
        Assert.Contains("public static extern conditionfunc Condition(Func<bool> func);", natives);
        Assert.Contains("public static extern @event TriggerRegisterTimerEvent(", natives);
        Assert.Contains("public static extern float RMinBJ(float a, float b);", natives);
    }

    [Fact]
    public void DefinesLuaTypesAfterTheirParents()
    {
        var lua = NativesWriter.WriteLuaTypes(Jass.Read(Common, Blizzard), "test");
        var lines = lua.Split('\n');

        int Line(string name) => Array.FindIndex(lines, l => l.Contains($"\"Moonwell.{name}\""));
        Assert.True(Line("handle") < Line("agent"));
        Assert.True(Line("agent") < Line("widget"));
        Assert.True(Line("widget") < Line("unit"));
        Assert.Contains("local t_handle = define(\"Moonwell.handle\", {})", lua);
        Assert.Contains("local t_unit = define(\"Moonwell.unit\", { base = { t_widget } })", lua);
    }
}
