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

    [Fact]
    public void a_lambda_given_through_a_delegate_parameter_is_a_local_block()
    {
        var found = MapCode.Check(
            """
            var unit = Unit.Of(u)!;
            ShowTo(p, () => unit.Kill());
            ShowTo(p, () => { var x = Natives.GetCameraTargetPositionX(); p.Print("x " + x); });
            Twice(p, () => Natives.KillUnit(u));
            RunInside(p, () => Natives.RemoveUnit(u));
            ShowTo(p, Kill);
            """,
            """
                public static void ShowTo(Player who, Action action) => Local.For(who, action);
                public static void Twice(Player who, Action action) => ShowTo(who, action);
                public static void RunInside(Player who, Action action) => Local.For(who, () => action());
                private static void Kill() => Natives.KillUnit(Natives.GetTriggerUnit());
            """);

        Assert.Equal(new[] { "MW0001: unit.Kill()", "MW0001: Natives.KillUnit(u)", "MW0001: Natives.RemoveUnit(u)", "MW0001: Kill" }, found);
    }

    [Fact]
    public void a_delegate_parameter_given_to_local_for_and_called_outside_is_mw0040()
    {
        var found = MapCode.Check("Both(p, () => { });", """
                public static void Both(Player who, Action action)
                {
                    action();
                    Local.For(who, action);
                }
            """);

        Assert.Equal(new[] { "MW0040: action" }, found);
    }

    [Fact]
    public void a_delegate_variable_field_or_event_is_followed()
    {
        var found = MapCode.Check(
            """
            Action kill = () => Natives.KillUnit(u);
            Local.For(p, kill);
            Local.For(p, OnShow);
            Local.For(p, () => Changed?.Invoke());
            Local.For(p, () => Fire());
            Action quiet = () => p.Print("hi");
            Local.For(p, quiet);
            """,
            """
                public static Action OnShow = () => Natives.KillUnit(Natives.GetTriggerUnit());
                public static event Action? Changed;
                public static Action? Later;

                public static void Wire()
                {
                    Changed += Spawn;
                    Later = () => Natives.CreateGroup();
                }

                private static void Spawn() => Natives.CreateUnit(Natives.Player(0), 0, 0f, 0f, 0f);

                private static void Fire() => Later?.Invoke();
            """);

        Assert.Equal(new[] { "MW0001: kill", "MW0001: OnShow", "MW0001: Changed", "MW0001: Fire()" }, found);
    }

    [Fact]
    public void code_the_analyzer_cannot_follow_inside_local_for_is_mw0039()
    {
        var found = MapCode.Check(
            """
            var actions = new List<Action>();
            Local.For(p, actions[0]);
            Local.For(p, () => actions[0]());
            Action<Action> run = a => Local.For(p, a);
            Local.For(p, Make());
            Local.For(p, () => Make()());
            """,
            """
                private static Action Make() => () => { };
            """);

        Assert.Equal(new[] { "MW0039: actions[0]", "MW0039: actions[0]()", "MW0039: a" }, found);
    }

    [Fact]
    public void code_the_analyzer_cannot_follow_says_what_to_write()
    {
        Assert.Equal(
            "'actions[0]' runs code the analyzer cannot follow; inside Local.For, write the lambda in place or call a method of the map",
            MapCode.Message("var actions = new List<Action>(); Local.For(p, actions[0]);"));
    }

    [Fact]
    public void overrides_and_implementations_are_followed()
    {
        var found = MapCode.Check(
            """
            View v = new Spawner();
            Local.For(p, () => v.Refresh());
            Panel w = new Bad();
            Local.For(p, () => w.Refresh());
            IShow s = new Shower();
            Local.For(p, () => s.Show());
            var quiet = new Quiet();
            Local.For(p, () => quiet.Refresh());
            """,
            """
                public class View { public virtual void Refresh() { } }
                public sealed class Spawner : View { public override void Refresh() { Natives.CreateUnit(Natives.Player(0), 0, 0f, 0f, 0f); } }
                public sealed class Quiet : View { public override void Refresh() { } }
                public abstract class Panel { public abstract void Refresh(); }
                public sealed class Bad : Panel { public override void Refresh() { Natives.KillUnit(Natives.GetTriggerUnit()); } }
                public interface IShow { void Show(); }
                public sealed class Shower : IShow { public void Show() => Natives.RemoveUnit(Natives.GetTriggerUnit()); }
            """);

        Assert.Equal(new[] { "MW0001: v.Refresh()", "MW0001: w.Refresh()", "MW0001: s.Show()" }, found);
    }

    [Fact]
    public void an_override_names_its_path()
    {
        Assert.Equal(
            "'View.Refresh' creates a handle (through View.Refresh > Spawner.Refresh > CreateGroup); inside Local.For it runs on one machine only, which desyncs the game",
            MapCode.Message("View v = new Spawner(); Local.For(p, () => v.Refresh());", """
                public class View { public virtual void Refresh() { } }
                public sealed class Spawner : View { public override void Refresh() { Natives.CreateGroup(); } }
            """));
    }

    [Fact]
    public void operators_dispose_enumerators_and_compound_reads_are_followed()
    {
        var found = MapCode.Check(
            """
            var a = new Pt();
            Local.For(p, () => { var b = a + a; });
            Local.For(p, () => { using (var k = new Killer()) { } });
            Local.For(p, () => { using var k = new Killer(); });
            Local.For(p, () => { foreach (var x in new Spawns()) { } });
            Local.For(p, () => { Score += 1; });
            Local.For(p, () => { int n = a; });
            """,
            """
                public sealed class Pt
                {
                    public static Pt operator +(Pt x, Pt y) { Natives.CreateGroup(); return x; }
                    public static implicit operator int(Pt x) { Natives.KillUnit(Natives.GetTriggerUnit()); return 0; }
                }

                public sealed class Killer : IDisposable { public void Dispose() { Natives.KillUnit(Natives.GetTriggerUnit()); } }

                public sealed class Spawns
                {
                    public Spawner GetEnumerator() => new Spawner();
                }

                public sealed class Spawner
                {
                    public unit Current => Natives.GetTriggerUnit();
                    public bool MoveNext() { Natives.CreateUnit(Natives.Player(0), 0, 0f, 0f, 0f); return false; }
                }

                private static int score;
                public static int Score { get { Natives.CreateGroup(); return score; } set { score = value; } }
            """);

        Assert.Equal(new[]
        {
            "MW0001: a + a",
            "MW0001: var k = new Killer()",
            "MW0001: using var k = new Killer();",
            "MW0001: new Spawns()",
            "MW0001: Score",
            "MW0001: a",
        }, found);
    }
}
