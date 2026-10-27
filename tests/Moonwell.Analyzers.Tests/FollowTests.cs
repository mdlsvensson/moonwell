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
}
