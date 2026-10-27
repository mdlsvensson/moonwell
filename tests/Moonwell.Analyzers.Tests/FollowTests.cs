namespace Moonwell.Analyzers.Tests;

/// <summary>MW0001 for what Local.For reaches: the map code it follows, whatever the order of the code.</summary>
public class FollowTests
{
    private const string Mutual = """
            private static void A(int n)
            {
                if (n > 0) { B(n - 1); }
                Natives.KillUnit(Natives.GetTriggerUnit());
            }

            private static void B(int n)
            {
                if (n > 0) { A(n - 1); }
            }
        """;

    [Theory]
    [InlineData("Local.For(p, () => A(1));\nLocal.For(p, () => B(1));")]
    [InlineData("Local.For(p, () => B(1));\nLocal.For(p, () => A(1));")]
    public void mutual_recursion_is_reported_whatever_comes_first(string body)
    {
        var found = MapCode.Check(body, Mutual);

        Assert.Equal(2, found.Count);
        Assert.All(found, f => Assert.StartsWith("MW0001: ", f));
    }

    [Fact]
    public void a_method_reached_again_through_a_longer_cycle_is_reported()
    {
        var found = MapCode.Check(
            """
            Local.For(p, () => C(1));
            Local.For(p, () => D(1));
            Local.For(p, () => E(1));
            """,
            """
                private static void C(int n) { if (n > 0) { D(n - 1); } Natives.KillUnit(Natives.GetTriggerUnit()); }
                private static void D(int n) { if (n > 0) { E(n - 1); } }
                private static void E(int n) { if (n > 0) { C(n - 1); } }
            """);

        Assert.Equal(new[] { "MW0001: C(1)", "MW0001: D(1)", "MW0001: E(1)" }, found);
    }

    [Fact]
    public void instance_initializers_are_followed_when_a_class_is_made_inside()
    {
        var found = MapCode.Check(
            """
            Local.For(p, () => { var f = new Fx(); });
            Local.For(p, () => { var g = new Grouped(); });
            Local.For(p, () => { var h = new Plain(); });
            Local.For(p, () => { var d = new Derived(); });
            """,
            """
                public sealed class Fx { public readonly effect E = Natives.AddSpecialEffect("x", 0f, 0f); }
                public sealed class Grouped { private readonly group g = Natives.CreateGroup(); public Grouped() { } }
                public sealed class Plain { public int X = 1; public effect? E { get; set; } }
                public class Base { protected group G { get; } = Natives.CreateGroup(); }
                public sealed class Derived : Base { }
            """);

        Assert.Equal(new[] { "MW0001: new Fx()", "MW0001: new Grouped()", "MW0001: new Derived()" }, found);
    }

    [Fact]
    public void static_initializers_are_followed_when_a_class_is_touched_inside()
    {
        var found = MapCode.Check(
            """
            Local.For(p, () => { var g = Pool.G; });
            Local.For(p, () => Hud.Show("hi"));
            Local.For(p, () => { var n = Counter.N; });
            Local.For(p, () => { var k = Safe.K + Safe.L.Length; });
            Local.For(p, () => { var o = Own; });
            """,
            """
                private static readonly group Own = Natives.CreateGroup();

                public static class Pool { public static readonly group G = Natives.CreateGroup(); }

                public static class Hud
                {
                    private static readonly framehandle Box = Natives.BlzCreateFrameByType("TEXT", "Box", Natives.BlzGetOriginFrame(Natives.ORIGIN_FRAME_GAME_UI, 0), "", 0);

                    public static void Show(string text) => Natives.BlzFrameSetText(Box, text);
                }

                public static class Counter
                {
                    public static int N;

                    static Counter() { Natives.CreateTimer(); }
                }

                public static class Safe { public const int K = 3; public static readonly int[] L = new int[4]; }
            """);

        // The class whose code runs is already there: its own statics are not touched for the first time.
        Assert.Equal(new[] { "MW0001: Pool.G", "MW0001: Hud.Show(\"hi\")", "MW0001: Counter.N" }, found);
    }

    [Fact]
    public void a_static_initializer_names_its_path()
    {
        Assert.Equal(
            "'Pool.G' creates a handle (through Pool.G > static Pool() > CreateGroup); inside Local.For it runs on one machine only, which desyncs the game",
            MapCode.Message(
                "Local.For(p, () => { var g = Pool.G; });",
                "public static class Pool { public static readonly group G = Natives.CreateGroup(); }"));
    }
}
