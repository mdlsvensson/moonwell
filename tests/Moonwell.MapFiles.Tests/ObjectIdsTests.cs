using Moonwell.GameData;
using Moonwell.MapFiles;

namespace Moonwell.MapFiles.Tests;

public class ObjectIdsTests
{
    [Fact]
    public void Ids_count_from_000()
    {
        Assert.Equal("h000", ObjectIds.Next('h', _ => false));

        var taken = new HashSet<string> { "h000", "h001", "h002", "h003", "h004", "h005", "h006", "h007", "h008", "h009" };
        Assert.Equal("h00A", ObjectIds.Next('h', taken.Contains));
        Assert.Equal("h00Z", ObjectIds.Next('h', id => string.CompareOrdinal(id, "h00Z") < 0));
        Assert.Equal("h010", ObjectIds.Next('h', id => id.StartsWith("h00", StringComparison.Ordinal)));
    }

    [Fact]
    public void All_taken_is_null()
    {
        Assert.Null(ObjectIds.Next('h', _ => true));
    }

    [Fact]
    public void A_typed_id_may_have_signs()
    {
        Assert.True(ObjectIds.IsValid("A00!"));
        Assert.True(ObjectIds.IsValid("h000"));
        Assert.False(ObjectIds.IsValid("A00 "));
        Assert.False(ObjectIds.IsValid("A0é0"));
        Assert.False(ObjectIds.IsValid("A00"));
        Assert.False(ObjectIds.IsValid("A0000"));
    }

    [Fact]
    public void The_letter_follows_the_kind_and_a_heros_base()
    {
        Assert.Equal('h', ObjectIds.Letter(Kind.Unit, "hfoo"));
        Assert.Equal('H', ObjectIds.Letter(Kind.Unit, "Hpal"));
        Assert.Equal('I', ObjectIds.Letter(Kind.Item, "ratc"));
        Assert.Equal('B', ObjectIds.Letter(Kind.Destructable, "LTlt"));
        Assert.Equal('D', ObjectIds.Letter(Kind.Doodad, "ATtr"));
        Assert.Equal('A', ObjectIds.Letter(Kind.Ability, "AHhb"));
        Assert.Equal('B', ObjectIds.Letter(Kind.Buff, "Bhea"));
        Assert.Equal('R', ObjectIds.Letter(Kind.Upgrade, "Rhme"));
    }

    [Fact]
    public void An_upper_case_unit_id_is_a_heros()
    {
        Assert.True(ObjectIds.IsHeroId("Hfoo"));
        Assert.False(ObjectIds.IsHeroId("hfoo"));
        Assert.False(ObjectIds.IsHeroId("1foo"));
    }
}
