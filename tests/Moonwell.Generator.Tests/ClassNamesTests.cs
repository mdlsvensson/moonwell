using Moonwell.Generator.Objects;

namespace Moonwell.Generator.Tests;

public class ClassNamesTests
{
    private static StandardObject Object(
        Kind kind,
        string id,
        string name,
        string race = "",
        bool campaign = false,
        string? properNames = null)
    {
        return new StandardObject(
            kind,
            id,
            name,
            race,
            campaign,
            kind == Kind.Unit && char.IsAsciiLetterUpper(id[0]),
            null,
            properNames == null ? new Dictionary<string, string>() : new Dictionary<string, string> { ["upro"] = properNames },
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

        Assert.Equal("Moonwell.Objects.Human.Units", name.Outer);
        Assert.Equal("Paladin", name.Name);
        Assert.Equal("Moonwell.Objects.Human.Units.Paladin", name.FullName);
    }

    [Fact]
    public void races_give_their_namespace()
    {
        Assert.Equal("Moonwell.Objects.NightElf.Units.Archer", FullName(Object(Kind.Unit, "earc", "Archer", "nightelf")));
        Assert.Equal("Moonwell.Objects.Neutral.Units.Peasant", FullName(Object(Kind.Unit, "npea", "Peasant", "commoner")));
        Assert.Equal("Moonwell.Objects.Creeps.Units.Gnoll", FullName(Object(Kind.Unit, "ngno", "Gnoll", "creeps")));
        Assert.Equal("Moonwell.Objects.Critters.Units.Sheep", FullName(Object(Kind.Unit, "nshe", "Sheep", "critters")));
        Assert.Equal("Moonwell.Objects.Other.Units.Thing", FullName(Object(Kind.Unit, "nthg", "Thing", "other")));
        Assert.Equal("Moonwell.Objects.Other.Units.Blank", FullName(Object(Kind.Unit, "nbln", "Blank")));
        Assert.Equal("Moonwell.Objects.Other.Units.Odd", FullName(Object(Kind.Unit, "nodd", "Odd", "mystery")));
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
            Object(Kind.Unit, "Hart", "Paladin", campaign: true, properNames: "Arthas"),
            Object(Kind.Unit, "Uear", "Lich", campaign: true, properNames: "Arthas"),
            Object(Kind.Unit, "hfoo", "Footman", "human", campaign: true),
        });

        Assert.Equal("Moonwell.Objects.Campaign.Units.ArthasHart", names[(Kind.Unit, "Hart")].FullName);
        Assert.Equal("Moonwell.Objects.Campaign.Units.ArthasUear", names[(Kind.Unit, "Uear")].FullName);
        Assert.Equal("Moonwell.Objects.Campaign.Units.Footman", names[(Kind.Unit, "hfoo")].FullName);
    }

    [Fact]
    public void campaign_namespace()
    {
        var arthas = Object(Kind.Unit, "Hart", "Paladin", "human", campaign: true, properNames: "Arthas");

        Assert.Equal("Moonwell.Objects.Campaign.Units.Arthas", FullName(arthas));
    }

    [Fact]
    public void a_hero_with_several_proper_names_keeps_its_class_name()
    {
        var paladin = Object(Kind.Unit, "Hpal", "Paladin", "human", properNames: "Granis Darkhammer,Jorn the Redeemer");

        Assert.Equal("Moonwell.Objects.Human.Units.Paladin", FullName(paladin));
    }

    [Fact]
    public void a_hero_with_one_proper_name_is_named_by_it()
    {
        Assert.Equal(
            "Moonwell.Objects.Campaign.Units.KelThuzad",
            FullName(Object(Kind.Unit, "Uktl", "Lich", "undead", true, " Kel'Thuzad ")));
        Assert.Equal("Moonwell.Objects.Human.Units.Paladin", FullName(Object(Kind.Unit, "Hxxx", "Paladin", "human", properNames: " ")));
        Assert.Equal("Moonwell.Objects.Human.Units.Paladin", FullName(Object(Kind.Unit, "Hyyy", "Paladin", "human", properNames: "'")));
        Assert.Equal("Moonwell.Objects.Human.Units.Paladin", FullName(Object(Kind.Unit, "Hzzz", "Paladin", "human", properNames: "Arthas,")));
    }

    [Fact]
    public void a_non_hero_with_one_proper_name_is_unaffected()
    {
        var unit = Object(Kind.Unit, "hfoo", "Footman", "human", properNames: "Arthas");

        Assert.Equal("Moonwell.Objects.Human.Units.Footman", FullName(unit));
    }

    [Fact]
    public void two_heroes_with_the_same_proper_name_both_get_ids()
    {
        var names = ClassNames.Assign(new[]
        {
            Object(Kind.Unit, "Hct1", "Guard Lieutenant", "human", properNames: "Garek"),
            Object(Kind.Unit, "Hctl", "Guard Captain", "human", properNames: "Garek"),
        });

        Assert.Equal("Garek", names[(Kind.Unit, "Hct1")].Name);
        Assert.Equal("GarekHctl", names[(Kind.Unit, "Hctl")].Name);
    }

    [Fact]
    public void defend_ability_and_upgrade_share_a_name_in_different_kinds()
    {
        var names = ClassNames.Assign(new[]
        {
            Object(Kind.Ability, "Adef", "Defend", "human"),
            Object(Kind.Upgrade, "Rhde", "Defend", "human"),
        });

        Assert.Equal("Moonwell.Objects.Human.Abilities.Defend", names[(Kind.Ability, "Adef")].FullName);
        Assert.Equal("Moonwell.Objects.Human.Upgrades.Defend", names[(Kind.Upgrade, "Rhde")].FullName);
    }

    [Fact]
    public void name_equal_to_a_container_gets_its_id()
    {
        Assert.Equal("Moonwell.Objects.Human.Units.UnitsHuns", FullName(Object(Kind.Unit, "huns", "Units", "human")));
        Assert.Equal("Moonwell.Objects.Orc.Abilities.OrcAorc", FullName(Object(Kind.Ability, "Aorc", "Orc", "orc")));
    }

    [Fact]
    public void reflection_name_joins_containers_with_plus()
    {
        var name = ClassNames.Assign(new[] { Object(Kind.Unit, "Hpal", "Paladin", "human") })[(Kind.Unit, "Hpal")];

        Assert.Equal("Moonwell.Objects.Human+Units+Paladin", name.ReflectionName);
        Assert.Equal("Moonwell.Objects.Items+ClawsOfAttack", ClassNames.Assign(new[] { Object(Kind.Item, "ratc", "Claws of Attack") })[(Kind.Item, "ratc")].ReflectionName);
    }

    [Fact]
    public void ability_and_buff_apart()
    {
        var ability = Object(Kind.Ability, "AHav", "Avatar", "human");
        var buff = Object(Kind.Buff, "BHav", "Avatar", "human");

        var names = ClassNames.Assign(new[] { ability, buff });

        Assert.Equal("Moonwell.Objects.Human.Abilities.Avatar", names[(Kind.Ability, "AHav")].FullName);
        Assert.Equal("Moonwell.Objects.Human.Buffs.Avatar", names[(Kind.Buff, "BHav")].FullName);
    }

    [Fact]
    public void upgrades_are_by_race_and_the_rest_are_flat()
    {
        Assert.Equal(
            "Moonwell.Objects.Orc.Upgrades.SpikedBarricades",
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
        Assert.Equal("Moonwell.Objects.Human.Buffs.BHxx", FullName(Object(Kind.Buff, "BHxx", "", "human")));
        Assert.Equal("Moonwell.Objects.Human.Buffs.BHxx", FullName(Object(Kind.Buff, "BHxx", "'", "human")));
        Assert.Equal("Moonwell.Objects.Doodads._0abc", FullName(Object(Kind.Doodad, "0abc", "")));
    }

    [Fact]
    public void names_are_valid_identifiers()
    {
        Assert.Equal("Moonwell.Objects.Campaign.Units.Guldan", FullName(Object(Kind.Unit, "Ogld", "Gul'dan", "orc", true)));
        Assert.Equal("Moonwell.Objects.Campaign.Units.KelThuzad", FullName(Object(Kind.Unit, "Uktl", "Kel'Thuzad", "undead", true)));
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
        Assert.Equal("Moonwell.Objects.Human.Units.HumanHhum", FullName(Object(Kind.Unit, "hhum", "Human", "human")));
        Assert.Equal("Moonwell.Objects.Items.ItemsItms", FullName(Object(Kind.Item, "itms", "Items")));
    }

    [Fact]
    public void a_kind_base_or_runtime_type_name_appends_id()
    {
        Assert.Equal("Moonwell.Objects.Human.Units.StandardUnitHstd", FullName(Object(Kind.Unit, "hstd", "Standard Unit", "human")));
        Assert.Equal("Moonwell.Objects.Human.Units.UnitHunt", FullName(Object(Kind.Unit, "hunt", "Unit", "human")));
        Assert.Equal("Moonwell.Objects.Human.Abilities.BuffAbuf", FullName(Object(Kind.Ability, "Abuf", "Buff", "human")));
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
