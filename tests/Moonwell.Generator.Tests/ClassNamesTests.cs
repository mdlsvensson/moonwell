using Moonwell.Generator.Objects;

namespace Moonwell.Generator.Tests;

public class ClassNamesTests
{
    private static StandardObject Object(
        Kind kind,
        string id,
        string name,
        string race = "",
        bool campaign = false)
    {
        return new StandardObject(
            kind,
            id,
            name,
            race,
            campaign,
            kind == Kind.Unit && char.IsAsciiLetterUpper(id[0]),
            null,
            new Dictionary<string, string>(),
            new Dictionary<string, IReadOnlyDictionary<string, string>>(),
            new List<string>());
    }

    private static string FullName(StandardObject o)
    {
        return ClassNames.Assign(new[] { o })[(o.Kind, o.Id)].FullName;
    }

    [Fact]
    public void paladin_is_human()
    {
        var paladin = Object(Kind.Unit, "Hpal", "Paladin", "human");

        var name = ClassNames.Assign(new[] { paladin })[(Kind.Unit, "Hpal")];

        Assert.Equal("Moonwell.Objects.Units.Human", name.Namespace);
        Assert.Equal("Paladin", name.Name);
        Assert.Equal("Moonwell.Objects.Units.Human.Paladin", name.FullName);
    }

    [Fact]
    public void races_give_their_namespace()
    {
        Assert.Equal("Moonwell.Objects.Units.NightElf.Archer", FullName(Object(Kind.Unit, "earc", "Archer", "nightelf")));
        Assert.Equal("Moonwell.Objects.Units.Neutral.Peasant", FullName(Object(Kind.Unit, "npea", "Peasant", "commoner")));
        Assert.Equal("Moonwell.Objects.Units.Creeps.Gnoll", FullName(Object(Kind.Unit, "ngno", "Gnoll", "creeps")));
        Assert.Equal("Moonwell.Objects.Units.Critters.Sheep", FullName(Object(Kind.Unit, "nshe", "Sheep", "critters")));
        Assert.Equal("Moonwell.Objects.Units.Other.Thing", FullName(Object(Kind.Unit, "nthg", "Thing", "other")));
        Assert.Equal("Moonwell.Objects.Units.Other.Blank", FullName(Object(Kind.Unit, "nbln", "Blank")));
        Assert.Equal("Moonwell.Objects.Units.Other.Odd", FullName(Object(Kind.Unit, "nodd", "Odd", "mystery")));
    }

    [Fact]
    public void collision_appends_id()
    {
        var footman = Object(Kind.Unit, "hafo", "Footman", "human");
        var other = Object(Kind.Unit, "hcth", "Footman", "human");

        var names = ClassNames.Assign(new[] { other, footman });

        Assert.Equal("Footman", names[(Kind.Unit, "hafo")].Name);
        Assert.Equal("FootmanHcth", names[(Kind.Unit, "hcth")].Name);
    }

    [Fact]
    public void collision_in_two_namespaces_is_none()
    {
        var names = ClassNames.Assign(new[]
        {
            Object(Kind.Unit, "hfoo", "Footman", "human"),
            Object(Kind.Unit, "ofoo", "Footman", "orc"),
        });

        Assert.Equal("Footman", names[(Kind.Unit, "hfoo")].Name);
        Assert.Equal("Footman", names[(Kind.Unit, "ofoo")].Name);
    }

    [Fact]
    public void a_campaign_object_never_keeps_the_plain_name()
    {
        var names = ClassNames.Assign(new[]
        {
            Object(Kind.Unit, "Hart", "Arthas", campaign: true),
            Object(Kind.Unit, "Uear", "Arthas", campaign: true),
            Object(Kind.Unit, "hfoo", "Footman", "human", campaign: true),
        });

        Assert.Equal("Moonwell.Objects.Units.Campaign.ArthasHart", names[(Kind.Unit, "Hart")].FullName);
        Assert.Equal("Moonwell.Objects.Units.Campaign.ArthasUear", names[(Kind.Unit, "Uear")].FullName);
        Assert.Equal("Moonwell.Objects.Units.Campaign.Footman", names[(Kind.Unit, "hfoo")].FullName);
    }

    [Fact]
    public void campaign_namespace()
    {
        var arthas = Object(Kind.Unit, "Hart", "Arthas", "human", campaign: true);

        Assert.Equal("Moonwell.Objects.Units.Campaign.Arthas", FullName(arthas));
    }

    [Fact]
    public void ability_and_buff_apart()
    {
        var ability = Object(Kind.Ability, "AHav", "Avatar", "human");
        var buff = Object(Kind.Buff, "BHav", "Avatar", "human");

        var names = ClassNames.Assign(new[] { ability, buff });

        Assert.Equal("Moonwell.Objects.Abilities.Human.Avatar", names[(Kind.Ability, "AHav")].FullName);
        Assert.Equal("Moonwell.Objects.Buffs.Human.Avatar", names[(Kind.Buff, "BHav")].FullName);
    }

    [Fact]
    public void upgrades_are_by_race_and_the_rest_are_flat()
    {
        Assert.Equal(
            "Moonwell.Objects.Upgrades.Orc.SpikedBarricades",
            FullName(Object(Kind.Upgrade, "Rosp", "Spiked Barricades", "orc")));
        Assert.Equal("Moonwell.Objects.Items.ClawsOfAttack", FullName(Object(Kind.Item, "ratc", "Claws of Attack", "")));
        Assert.Equal("Moonwell.Objects.Destructables.Tree", FullName(Object(Kind.Destructable, "LTlt", "Tree")));
        Assert.Equal("Moonwell.Objects.Doodads.Rock", FullName(Object(Kind.Doodad, "ABrk", "Rock")));
    }

    [Fact]
    public void the_same_id_in_two_kinds_is_two_classes()
    {
        var names = ClassNames.Assign(new[]
        {
            Object(Kind.Item, "ratf", "Claws of Attack"),
            Object(Kind.Doodad, "ratf", "Claws of Attack"),
        });

        Assert.Equal("Moonwell.Objects.Items.ClawsOfAttack", names[(Kind.Item, "ratf")].FullName);
        Assert.Equal("Moonwell.Objects.Doodads.ClawsOfAttack", names[(Kind.Doodad, "ratf")].FullName);
    }

    [Fact]
    public void nameless_uses_id()
    {
        Assert.Equal("Moonwell.Objects.Buffs.Human.BHxx", FullName(Object(Kind.Buff, "BHxx", "", "human")));
        Assert.Equal("Moonwell.Objects.Buffs.Human.BHxx", FullName(Object(Kind.Buff, "BHxx", "'", "human")));
        Assert.Equal("Moonwell.Objects.Doodads._0abc", FullName(Object(Kind.Doodad, "0abc", "")));
    }

    [Fact]
    public void names_are_valid_identifiers()
    {
        Assert.Equal("Moonwell.Objects.Units.Campaign.Guldan", FullName(Object(Kind.Unit, "Ogld", "Gul'dan", "orc", true)));
        Assert.Equal("Moonwell.Objects.Units.Campaign.KelThuzad", FullName(Object(Kind.Unit, "Uktl", "Kel'Thuzad", "undead", true)));
        Assert.Equal("Moonwell.Objects.Items._100Gold", FullName(Object(Kind.Item, "gold", "100 Gold")));
    }

    [Fact]
    public void keyword_appends_id()
    {
        Assert.Equal("Moonwell.Objects.Doodads.boolBool", FullName(Object(Kind.Doodad, "bool", "")));
        Assert.Equal("Moonwell.Objects.Doodads.thisThis", FullName(Object(Kind.Doodad, "this", "")));
    }

    [Fact]
    public void the_namespaces_last_part_appends_id()
    {
        Assert.Equal("Moonwell.Objects.Units.Human.HumanHhum", FullName(Object(Kind.Unit, "hhum", "Human", "human")));
        Assert.Equal("Moonwell.Objects.Items.ItemsItms", FullName(Object(Kind.Item, "itms", "Items")));
    }

    [Fact]
    public void a_kind_base_or_runtime_type_name_appends_id()
    {
        Assert.Equal("Moonwell.Objects.Units.Human.StandardUnitHstd", FullName(Object(Kind.Unit, "hstd", "Standard Unit", "human")));
        Assert.Equal("Moonwell.Objects.Units.Human.UnitHunt", FullName(Object(Kind.Unit, "hunt", "Unit", "human")));
        Assert.Equal("Moonwell.Objects.Abilities.Human.BuffAbuf", FullName(Object(Kind.Ability, "Abuf", "Buff", "human")));
        Assert.Equal("Moonwell.Objects.Items.MapImap", FullName(Object(Kind.Item, "imap", "Map")));
    }

    [Fact]
    public void an_appended_name_that_is_taken_is_an_error()
    {
        var error = Assert.Throws<InvalidOperationException>(() => ClassNames.Assign(new[]
        {
            Object(Kind.Unit, "hafo", "Footman", "human"),
            Object(Kind.Unit, "hcth", "Footman", "human"),
            Object(Kind.Unit, "hxxx", "Footman Hcth", "human"),
        }));

        Assert.Contains("FootmanHcth", error.Message);
    }
}
