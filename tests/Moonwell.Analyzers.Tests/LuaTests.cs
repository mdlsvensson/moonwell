namespace Moonwell.Analyzers.Tests;

/// <summary>MW0008 and MW0010: C# that CSharp.lua cannot compile, or compiles to Lua that does not do what .NET does.</summary>
public class LuaTests
{
    [Fact]
    public void collection_expressions_are_accepted_wherever_they_are()
    {
        Assert.Empty(MapCode.Check("""
            List<int> a = [1, 2];
            a = [];
            var b = new List<int>([..a, 3]);
            Take([4]);
            """, """
                static int[] Field = [5];
                static void Take(List<int> list) { }
            """));
    }

    [Fact]
    public void collection_initializers_and_indexers_are_accepted()
    {
        Assert.Empty(MapCode.Check("""
            var a = new List<int> { 1, 2 };
            List<int> b = new() { 3 };
            var c = new[] { 4 };
            var d = a[0] + c[0];
            """));
    }

    [Fact]
    public void using_static_natives_is_rejected()
    {
        Assert.Equal(new[] { "MW0008: using static Moonwell.Natives;", "MW0008: using static Natives;" },
            MapCode.CheckFile("""
                using static Moonwell.Natives;

                namespace Moonwell.Map
                {
                    using static Natives;

                    public static class Code
                    {
                        public static int Run() => GetRandomInt(1, 2);
                    }
                }
                """));
    }

    [Fact]
    public void using_static_natives_message_gives_what_to_write()
    {
        Assert.Equal(
            "using static Moonwell.Natives hides Moonwell.Player behind Natives.Player; write Natives.X",
            Assert.Single(MapCode.MessagesOfFile("""
                using static Moonwell.Natives;

                public static class Code
                {
                }
                """)));
    }

    [Fact]
    public void other_static_imports_and_natives_by_name_are_accepted()
    {
        Assert.Empty(MapCode.CheckFile("""
            using static System.Math;
            using Moonwell;

            public static class Code
            {
                public static float Run() => Max(1f, 2f) + Natives.GetRandomInt(1, 2);
            }
            """));
    }

    [Fact]
    public void a_name_of_a_choice_enum_is_rejected()
    {
        // In the game's Lua the enums of Moonwell.Objects are numbers only: ToString and a joined or interpolated string give 1, Enum.GetValues stops.
        Assert.Equal(new[]
        {
            "MW0010: armor.ToString()",
            "MW0010: armor",
            "MW0010: {armor}",
            "MW0010: Enum.GetValues(typeof(Moonwell.Objects.ArmorType))",
            "MW0010: Enum.GetNames<Moonwell.Objects.Targets>()",
            "MW0010: Enum.IsDefined(armor)",
            "MW0010: Enum.TryParse(\"metal\", out armor)",
        }, MapCode.Check("""
            var armor = Moonwell.Objects.ArmorType.Metal;
            var a = armor.ToString();
            var b = "armor " + armor;
            var c = $"{armor}";
            var d = Enum.GetValues(typeof(Moonwell.Objects.ArmorType));
            var e = Enum.GetNames<Moonwell.Objects.Targets>();
            var f = Enum.IsDefined(armor);
            var g = Enum.TryParse("metal", out armor);
            """));
    }

    [Fact]
    public void a_choice_enum_shown_through_an_object_a_compound_assignment_or_a_conditional_access_is_rejected()
    {
        // Each of these makes the string with the enum's ToString in .NET; in the game's Lua the enum is only its number.
        Assert.Equal(new[]
        {
            "MW0010: choice",
            "MW0010: choice",
            "MW0010: choice",
            "MW0010: choice",
            "MW0010: choice",
            "MW0010: choice",
            "MW0010: choice?.ToString()",
            "MW0010: choice",
            "MW0010: {choice}",
            "MW0010: choice.ToString()",
        }, MapCode.Check("""
            Moonwell.Objects.ArmorType? choice = Moonwell.Objects.ArmorType.Metal;
            var s = "armor ";
            s += choice;
            var a = string.Format("{0}", choice);
            var b = string.Concat("a", choice);
            var c = new System.Text.StringBuilder().Append(choice);
            Show(choice);
            var d = string.Format("{0} {1}", 1, choice);
            var e = choice?.ToString();
            var f = "armor " + choice;
            var g = $"{choice}";
            var h = choice.ToString();
            """, """
                static void Show(object value) { }
            """));
    }

    [Fact]
    public void a_choice_enum_given_to_an_object_parameter_is_reported_once_and_equals_is_accepted()
    {
        Assert.Equal(new[] { "MW0010: Enum.IsDefined(typeof(Moonwell.Objects.ArmorType), armor)" }, MapCode.Check("""
            var armor = Moonwell.Objects.ArmorType.Metal;
            var a = Enum.IsDefined(typeof(Moonwell.Objects.ArmorType), armor);
            var b = armor.Equals(Moonwell.Objects.ArmorType.Stone) || object.Equals(armor, Moonwell.Objects.ArmorType.Stone);
            var c = new System.Text.StringBuilder().Append((int)armor).Append(DayOfWeek.Monday);
            Show((int)armor);
            """, """
                static void Show(object value) { }
            """));
    }

    [Fact]
    public void a_choice_enum_message_says_why_and_what_to_write()
    {
        Assert.Equal(
            "'armor.ToString()' needs the names of ArmorType, which the game's Lua does not have: there a choice is only its number (ArmorType.Metal shows as 1) and Enum.GetValues stops; compare with its members, or show (int) of it",
            MapCode.Message("var armor = Moonwell.Objects.ArmorType.Metal; var a = armor.ToString();"));
    }

    [Fact]
    public void comparing_combining_and_has_flag_on_a_choice_enum_and_names_of_other_enums_are_accepted()
    {
        Assert.Empty(MapCode.Check("""
            var armor = Moonwell.Objects.ArmorType.Metal;
            var targets = Moonwell.Objects.Targets.Air | Moonwell.Objects.Targets.Enemies;
            var a = armor == Moonwell.Objects.ArmorType.Stone || targets.HasFlag(Moonwell.Objects.Targets.Air) || (targets & Moonwell.Objects.Targets.Enemies) != 0;
            var b = "armor " + (int)armor + $"{(int)targets}";
            var c = DayOfWeek.Monday.ToString() + Enum.GetNames(typeof(DayOfWeek)).Length;
            """));
    }

    [Fact]
    public void names_of_the_core_attack_and_defense_types_are_accepted()
    {
        // AttackType and DefenseType are Moonwell's own: CSharp.lua compiles them, so the game's Lua has their names.
        Assert.Empty(MapCode.Check("""
            var attack = AttackType.Magic;
            var a = AttackType.Magic.ToString() + " " + DefenseType.Divine + $"{attack}";
            """));
    }
}
