namespace Moonwell.Analyzers.Tests;

/// <summary>
/// MW0038 and MW0002 for .NET's own machine-local values: Random without a seed, the clock, and an object's address
/// (GetHashCode, a handle as text). In the game's Lua math.random is the game's synced generator, and CoreSystem's
/// Random without a seed reseeds it with os.time() (seen 2026-10-26).
/// </summary>
public class ClockTests
{
    [Fact]
    public void random_without_a_seed_is_rejected_everywhere()
    {
        var found = MapCode.Check("""
            var r = new Random();
            Random s = new();
            Local.For(p, () => { var t = new Random(); });
            var seeded = new Random(Natives.GetRandomInt(0, 1000)).Next(0, 9);
            """);

        Assert.Equal(new[] { "MW0038: new Random()", "MW0038: new()", "MW0038: new Random()" }, found);
    }

    [Fact]
    public void random_without_a_seed_says_what_to_write()
    {
        Assert.Equal(
            "'Random' seeds itself from this machine's clock and reseeds the game's random generator with it, which desyncs every random number after it, inside Local.For too; use Natives.GetRandomInt, or new Random(seed) with a seed every machine shares",
            MapCode.Message("var r = new Random();"));
    }

    [Fact]
    public void random_shared_is_not_in_the_lua_core()
    {
        Assert.Equal(new[] { "MW0030: Shared" }, MapCode.Check("var r = Random.Shared.Next(0, 9);"));
    }

    [Fact]
    public void the_clock_is_rejected_outside_and_read_inside()
    {
        var found = MapCode.Check("""
            var now = DateTime.Now.Second;
            var utc = DateTime.UtcNow;
            var day = DateTime.Today;
            var tick = Environment.TickCount;
            if (Environment.TickCount64 > 0) { }
            var watch = System.Diagnostics.Stopwatch.StartNew();
            var seconds = watch.Elapsed.Seconds;
            if (System.Diagnostics.Stopwatch.GetTimestamp() > 0) { }
            Local.For(p, () =>
            {
                var shown = DateTime.Now.Second + Environment.TickCount + watch.Elapsed.Seconds;
            });
            """);

        Assert.Equal(new[]
        {
            "MW0002: DateTime.Now",
            "MW0002: DateTime.UtcNow",
            "MW0002: DateTime.Today",
            "MW0002: Environment.TickCount",
            "MW0002: Environment.TickCount64",
            "MW0002: watch.Elapsed",
            "MW0002: System.Diagnostics.Stopwatch.GetTimestamp()",
        }, found);
    }

    [Fact]
    public void the_clock_says_what_to_write()
    {
        Assert.Equal(
            "'DateTime.Now' gives a different answer on each machine; use it only inside Local.For or a [MachineLocal] method; it is this machine's clock: time game logic with Timer.After or Timer.Every",
            MapCode.Message("var now = DateTime.Now.Second;"));
    }

    [Fact]
    public void an_address_is_rejected_outside_and_read_inside()
    {
        var found = MapCode.Check(
            """
            var a = u.GetHashCode();
            var b = new Fx().GetHashCode();
            var c = new Keyed().GetHashCode() + "x".GetHashCode() + 5.GetHashCode() + new Pt().GetHashCode();
            var d = System.Runtime.CompilerServices.RuntimeHelpers.GetHashCode(u);
            var e = u.ToString();
            var f = $"{u}";
            var g = "unit " + u;
            var h = new Fx().ToString() + p.ToString();
            Local.For(p, () =>
            {
                var shown = u.GetHashCode() + u.ToString() + $"{u}" + "unit " + u;
            });
            """,
            """
                public sealed class Fx { }
                public sealed class Keyed { public override int GetHashCode() => 1; }
                public struct Pt { public int X; }
            """);

        Assert.Equal(new[]
        {
            "MW0002: u.GetHashCode()",
            "MW0002: new Fx().GetHashCode()",
            "MW0002: System.Runtime.CompilerServices.RuntimeHelpers.GetHashCode(u)",
            "MW0002: u.ToString()",
            "MW0002: u",
            "MW0002: u",
        }, found);
    }

    [Fact]
    public void an_address_says_what_to_write()
    {
        Assert.Equal(new[]
        {
            "'Object.GetHashCode' gives a different answer on each machine; use it only inside Local.For or a [MachineLocal] method; in the game's Lua it is the object's address: key by the object itself, or by a number the map assigns",
            "'u' gives a different answer on each machine; use it only inside Local.For or a [MachineLocal] method; in the game's Lua a handle as text is its address",
        }, MapCode.MessagesOfFile("""
            using Moonwell;

            namespace Map
            {
                public static class Use
                {
                    public static string Run(unit u) => u.GetHashCode() + " " + u;
                }
            }
            """));
    }

    [Fact]
    public void sync_unchecked_silences_them()
    {
        Assert.Empty(MapCode.Check("Clock(u);", """
                [SyncUnchecked]
                private static void Clock(unit u)
                {
                    var r = new Random().Next();
                    var now = DateTime.Now.Second + u.GetHashCode();
                }
            """));
    }
}
