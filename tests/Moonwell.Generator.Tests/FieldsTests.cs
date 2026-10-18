using Moonwell.Generator.Objects;

namespace Moonwell.Generator.Tests;

public class FieldsTests
{
    private static ExportTables MiniExport()
    {
        return ExportTables.Read(Path.GetFullPath(
            Path.Combine(AppContext.BaseDirectory, "..", "..", "..", "Fixtures", "MiniExport", "war3.w3mod")));
    }

    private static Field Find(IReadOnlyList<Field> fields, string id)
    {
        return fields.Single(f => f.Id == id);
    }

    [Theory]
    [InlineData(Kind.Unit, "uhpm", "HitPoints")]
    [InlineData(Kind.Unit, "unam", "Name")]
    [InlineData(Kind.Unit, "umvs", "Speed")]
    [InlineData(Kind.Unit, "udef", "Armor")]
    [InlineData(Kind.Unit, "uabi", "Abilities")]
    [InlineData(Kind.Ability, "acdn", "Cooldown")]
    [InlineData(Kind.Ability, "amcs", "ManaCost")]
    [InlineData(Kind.Ability, "alev", "LevelCount")]
    [InlineData(Kind.Upgrade, "glvl", "LevelCount")]
    [InlineData(Kind.Ability, "anam", "Name")]
    [InlineData(Kind.Ability, "abuf", "Buffs")]
    [InlineData(Kind.Ability, "atar", "Targets")]
    [InlineData(Kind.Item, "iabi", "Abilities")]
    public void hand_picked_names(Kind kind, string id, string name)
    {
        Assert.Equal(name, FieldNames.HandPicked(kind)[id]);
        Assert.Equal(name, Find(Fields.Of(MiniExport(), kind), id).Name);
    }

    [Fact]
    public void every_listed_id_exists()
    {
        var tables = MiniExport();

        foreach (var kind in Enum.GetValues<Kind>())
        {
            var ids = Fields.Of(tables, kind).Select(f => f.Id).ToHashSet();
            foreach (var id in FieldNames.HandPicked(kind).Keys)
            {
                Assert.True(ids.Contains(id), $"{kind} {id} is listed in field-names.txt but is not a field of the kind");
            }
        }
    }

    [Fact]
    public void types_from_metadata()
    {
        var tables = MiniExport();

        var hitPoints = Find(Fields.Of(tables, Kind.Unit), "uhpm");
        Assert.Equal(FieldType.Int, hitPoints.Type);
        Assert.Null(hitPoints.ListOf);
        Assert.False(hitPoints.Skin);

        var abilities = Find(Fields.Of(tables, Kind.Unit), "uabi");
        Assert.Equal(FieldType.ObjectList, abilities.Type);
        Assert.Equal("Ability", abilities.ListOf);

        var name = Find(Fields.Of(tables, Kind.Unit), "unam");
        Assert.Equal(FieldType.String, name.Type);
        Assert.True(name.Skin);

        Assert.Equal(FieldType.Float, Find(Fields.Of(tables, Kind.Unit), "uhpr").Type);
        Assert.Equal(FieldType.Bool, Find(Fields.Of(tables, Kind.Ability), "aher").Type);
        Assert.Equal("object", Find(Fields.Of(tables, Kind.Unit), "ureq").ListOf);
        Assert.Equal("Unit", Find(Fields.Of(tables, Kind.Unit), "ubui").ListOf);
    }

    [Fact]
    public void per_level_and_pointer()
    {
        var field = Find(Fields.Of(MiniExport(), Kind.Ability), "Htb1");

        Assert.True(field.PerLevel);
        Assert.Equal(1, field.DataPointer);
        Assert.Contains("AHtb", field.Specific);
        Assert.Equal(FieldType.Float, field.Type);
        Assert.Equal("DataA{level}", field.Column);
        Assert.False(Find(Fields.Of(MiniExport(), Kind.Ability), "alev").PerLevel);
        Assert.Empty(Find(Fields.Of(MiniExport(), Kind.Ability), "alev").Specific);
        Assert.Equal(0, Find(Fields.Of(MiniExport(), Kind.Ability), "acdn").DataPointer);
    }

    [Fact]
    public void columns_find_the_values()
    {
        var tables = MiniExport();

        Assert.Equal("HP", Find(Fields.Of(tables, Kind.Unit), "uhpm").Column);
        Assert.Equal("Cool{level}", Find(Fields.Of(tables, Kind.Ability), "acdn").Column);
        Assert.Equal("Tip", Find(Fields.Of(tables, Kind.Ability), "atp1").Column);
        Assert.True(Find(Fields.Of(tables, Kind.Ability), "atp1").PerLevel);
        Assert.Equal("Buttonpos[0]", Find(Fields.Of(tables, Kind.Unit), "ubpx").Column);
        Assert.Equal("Buttonpos[1]", Find(Fields.Of(tables, Kind.Unit), "ubpy").Column);
        Assert.Equal("Buttonpos[1]", Find(Fields.Of(tables, Kind.Ability), "abpy").Column);
        Assert.Equal("Name", Find(Fields.Of(tables, Kind.Unit), "unam").Column);
        Assert.Equal("Requires", Find(Fields.Of(tables, Kind.Upgrade), "greq").Column);
        Assert.Equal("Name", Find(Fields.Of(tables, Kind.Destructable), "bnam").Column);
    }

    [Fact]
    public void units_and_items_take_their_own_fields()
    {
        var tables = MiniExport();
        var units = Fields.Of(tables, Kind.Unit).Select(f => f.Id).ToList();
        var items = Fields.Of(tables, Kind.Item).Select(f => f.Id).ToList();

        Assert.Contains("uabi", units);
        Assert.DoesNotContain("uabi", items);
        Assert.Contains("iabi", items);
        Assert.DoesNotContain("iabi", units);
        Assert.Contains("unam", units);
        Assert.Contains("unam", items);
        Assert.All(Fields.Of(tables, Kind.Item), f => Assert.Equal(Kind.Item, f.Kind));
        Assert.Equal(units.OrderBy(i => i, StringComparer.Ordinal), units);
    }

    [Fact]
    public void name_from_a_metadata_label()
    {
        var tables = MiniExport();

        Assert.Equal("ButtonPositionX", Find(Fields.Of(tables, Kind.Unit), "ubpx").Name);
        Assert.Equal("RequirementsTier2", Find(Fields.Of(tables, Kind.Unit), "urq1").Name);
    }

    [Theory]
    [InlineData("Hit Points Maximum (Base)", "HitPointsMaximumBase")]
    [InlineData("Area of Effect", "AreaOfEffect")]
    [InlineData("Caster's Level", "CastersLevel")]
    [InlineData("Damage/Healing - Bonus", "DamageHealingBonus")]
    [InlineData("1st Attack", "_1stAttack")]
    [InlineData("Attack 1 - Range", "Attack1Range")]
    [InlineData("  spaced   out  ", "SpacedOut")]
    [InlineData("(Not used)", "NotUsed")]
    [InlineData("%", "")]
    [InlineData("", "")]
    public void name_from_label(string label, string name)
    {
        Assert.Equal(name, FieldNames.FromLabel(label));
    }

    [Fact]
    public void colliding_name_gets_id()
    {
        var names = FieldNames.Assign(new[]
        {
            new NameCandidate("aare", "AreaOfEffect", false, new[] { "common", "AHbz" }),
            new NameCandidate("Hbz2", "AreaOfEffect", false, new[] { "AHbz" }),
            new NameCandidate("Hwe2", "AreaOfEffect", false, new[] { "AHwe" }),
            new NameCandidate("aran", "Range", true, new[] { "common", "AHbz" }),
            new NameCandidate("Hbz3", "Range", false, new[] { "AHbz" }),
        });

        Assert.Equal(new[] { "AreaOfEffectAare", "AreaOfEffectHbz2", "AreaOfEffect", "Range", "RangeHbz3" }, names);
    }

    [Fact]
    public void a_name_of_object_gets_its_id()
    {
        var names = FieldNames.Assign(new[] { new NameCandidate("utyp", "GetType", false, new[] { "unit" }) });

        Assert.Equal(new[] { "GetTypeUtyp" }, names);
    }

    [Theory]
    [InlineData("Declared")]
    [InlineData("Fields")]
    [InlineData("Levels")]
    [InlineData("PerLevel")]
    [InlineData("Set")]
    public void a_name_of_a_kind_base_member_gets_its_id(string name)
    {
        var names = FieldNames.Assign(new[] { new NameCandidate("ulev", name, false, new[] { "unit" }) });

        Assert.Equal(new[] { name + "Ulev" }, names);
    }

    [Theory]
    [InlineData("Set")]
    [InlineData("Per Level")]
    public void a_label_that_gives_a_kind_base_member_gets_its_id(string label)
    {
        var name = FieldNames.FromLabel(label);
        var names = FieldNames.Assign(new[] { new NameCandidate("abcd", name, false, new[] { "all" }) });

        Assert.Equal(name + "Abcd", names[0]);
    }

    [Fact]
    public void two_listed_names_that_meet_are_an_error()
    {
        var error = Assert.Throws<InvalidOperationException>(() => FieldNames.Assign(new[]
        {
            new NameCandidate("uhpm", "Life", true, new[] { "unit" }),
            new NameCandidate("uhpr", "Life", true, new[] { "unit" }),
        }));

        Assert.Equal("fields uhpm and uhpr are both named Life", error.Message);
    }

    [Fact]
    public void a_name_of_a_runtime_member_gets_its_id()
    {
        var names = FieldNames.Assign(
            new[]
            {
                new NameCandidate("uhnd", "Handle", false, new[] { "unit" }),
                new NameCandidate("uply", "Player", false, new[] { "unit" }),
                new NameCandidate("uabc", "Other", false, new[] { "unit" }),
            },
            new HashSet<string> { "Handle", "Player" });

        Assert.Equal(new[] { "HandleUhnd", "PlayerUply", "Other" }, names);
    }

    [Fact]
    public void a_hand_picked_runtime_member_name_is_an_error()
    {
        var error = Assert.Throws<InvalidOperationException>(() => FieldNames.Assign(
            new[] { new NameCandidate("uhpm", "Handle", true, new[] { "unit" }) },
            new HashSet<string> { "Handle" }));

        Assert.Equal("field uhpm is hand-picked as Handle, a member of the class it belongs to", error.Message);
    }

    [Fact]
    public void fields_of_a_kind_avoid_the_reserved_names()
    {
        var tables = MiniExport();

        var plain = Find(Fields.Of(tables, Kind.Unit), "ubpx");
        var avoided = Find(Fields.Of(tables, Kind.Unit, new HashSet<string> { plain.Name }), "ubpx");

        Assert.Equal("ButtonPositionX", plain.Name);
        Assert.Equal("ButtonPositionXUbpx", avoided.Name);
    }

    [Fact]
    public void a_hero_only_unit_field_is_marked()
    {
        var fields = Fields.Of(MiniExport(), Kind.Unit);

        Assert.True(Find(fields, "ustr").HeroOnly);
        Assert.True(Find(fields, "uhab").HeroOnly);
        Assert.False(Find(fields, "uhpm").HeroOnly);
        Assert.False(Find(fields, "uabi").HeroOnly);
        Assert.All(Fields.Of(MiniExport(), Kind.Ability), f => Assert.False(f.HeroOnly));
    }
}
