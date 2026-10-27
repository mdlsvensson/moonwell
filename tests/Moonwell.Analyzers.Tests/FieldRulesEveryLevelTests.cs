using System.Reflection;
using Moonwell.Objects;

namespace Moonwell.Analyzers.Tests;

/// <summary>
/// MW0009 on one value for every level (<c>Cooldown = 3900f;</c>, the implicit conversion to <c>Levels&lt;T&gt;</c>): the build
/// checks it at each level when it counts the levels, so the text is the one it gives at level 1.
/// </summary>
public class FieldRulesEveryLevelTests
{
    private const string StormBolt = "Moonwell.Objects.Human.Abilities.StormBolt";

    private static List<string> Messages(string statements) => MapCode.MessagesOfFile(Source(statements));

    private static string Source(string statements) => FieldRulesTests.File(FieldRulesTests.Class("Bolt", StormBolt, statements));

    [Fact]
    public void one_value_for_every_level_out_of_range_is_reported_at_level_1()
    {
        Assert.Equal(
            new[] { "Bolt.Cooldown at level 1 is 3900; Cooldown (acdn) allows 0 to 3600; [BeyondLimits] goes past it" },
            Messages("Cooldown = 3900f;"));
    }

    [Fact]
    public void one_whole_number_for_every_level_of_a_float_field_is_reported()
    {
        Assert.Equal(
            new[] { "Bolt.Cooldown at level 1 is 4000; Cooldown (acdn) allows 0 to 3600; [BeyondLimits] goes past it" },
            Messages("Cooldown = 4000;"));
    }

    [Fact]
    public void one_value_for_every_level_within_range_is_not_reported()
    {
        Assert.Empty(Messages("Cooldown = 5f;"));
    }

    [Theory]
    [InlineData("Cooldown = 3900f;")]
    [InlineData("Cooldown = -1f;")]
    [InlineData("Cooldown = 4000;")]
    public void the_text_is_the_one_the_build_gives_when_it_counts_the_levels(string statement)
    {
        var bolt = Activator.CreateInstance(MapCode.Load(Source(statement)).GetType("Map.Bolt")!)!;
        var levels = DeclaredOf(bolt)["acdn"]!;

        var thrown = Assert.Throws<TargetInvocationException>(
            () => levels.GetType().GetMethod("Resolve", BindingFlags.Instance | BindingFlags.NonPublic)!.Invoke(levels, new object[] { 3 }));

        Assert.Equal(new[] { Assert.IsType<FieldRuleException>(thrown.InnerException).Message }, Messages(statement));
    }

    private static IReadOnlyDictionary<string, object?> DeclaredOf(object made)
    {
        for (var type = made.GetType(); type != null; type = type.BaseType)
        {
            var property = type.GetProperty("Declared", BindingFlags.Instance | BindingFlags.NonPublic | BindingFlags.DeclaredOnly);
            if (property != null)
            {
                return (IReadOnlyDictionary<string, object?>)property.GetValue(made)!;
            }
        }

        throw new InvalidOperationException(made.GetType() + " has no Declared");
    }
}
