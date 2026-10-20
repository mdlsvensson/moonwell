namespace Moonwell.Analyzers.Tests;

/// <summary>MW0009: a constant a map class puts outside World Editor's limits, with the text the build gives.</summary>
public class FieldRulesTests
{
    private const string Header = """
        using System;
        using Moonwell;
        using Moonwell.Objects;

        namespace Map
        {
        """;

    internal static string File(string classes) => Header + "\n" + classes + "\n}\n";

    private static List<string> Messages(string classes) => MapCode.MessagesOfFile(File(classes));

    private static List<string> Found(string classes) => MapCode.CheckFile(File(classes));

    /// <summary>A class Giant of the standard footman whose constructor runs the statements.</summary>
    private static string Footman(string statements, string attribute = "") => Class("Giant", "Moonwell.Objects.Human.Units.Footman", statements, attribute);

    internal static string Class(string name, string @base, string statements, string attribute = "") => $$"""
        {{attribute}}
        public class {{name}} : {{@base}}
        {
            public {{name}}()
            {
                {{statements}}
            }
        }
        """;

    private static string Bolt(string statements) => Class("Bolt", "Moonwell.Objects.Human.Abilities.StormBolt", statements);

    private static string Archmage(string statements, string attribute = "") => Class("Mage", "Moonwell.Objects.Human.Units.Archmage", statements, attribute);

    private const string HitPointsText = "Giant.HitPoints is 600000; Hit Points Maximum (Base) (uhpm) allows 1 to 500000; [BeyondLimits] goes past it";

    [Fact]
    public void a_constant_out_of_range_in_an_assignment_gives_the_build_text()
    {
        Assert.Equal(new[] { HitPointsText }, Messages(Footman("HitPoints = 600000;")));
    }

    [Fact]
    public void the_diagnostic_is_mw0009_on_the_value()
    {
        Assert.Equal(new[] { "MW0009: 600000" }, Found(Footman("HitPoints = 600000;")));
    }

    [Fact]
    public void a_constant_out_of_range_in_levels_names_the_level()
    {
        Assert.Equal(
            new[] { "Bolt.Cooldown at level 3 is 3900; Cooldown (acdn) allows 0 to 3600; [BeyondLimits] goes past it" },
            Messages(Bolt("Cooldown = Levels(1f, 2f, 3900f);")));
    }

    [Fact]
    public void only_the_first_level_out_of_range_is_reported_and_it_is_the_lowest()
    {
        Assert.Equal(new[] { "MW0009: -1f" }, Found(Bolt("Cooldown = Levels(1f, -1f, 3900f);")));
    }

    [Fact]
    public void an_int_constant_in_levels_is_a_float()
    {
        Assert.Equal(
            new[] { "Bolt.Cooldown at level 2 is 4000; Cooldown (acdn) allows 0 to 3600; [BeyondLimits] goes past it" },
            Messages(Bolt("Cooldown = Levels(1f, 4000);")));
    }

    [Fact]
    public void a_constant_out_of_range_in_set_by_id_is_rejected()
    {
        Assert.Equal(new[] { HitPointsText }, Messages(Footman("""Set("uhpm", 600000);""")));
    }

    [Fact]
    public void a_constant_out_of_range_in_set_by_id_and_level_names_the_level()
    {
        Assert.Equal(
            new[] { "Bolt.Cooldown at level 2 is 3900; Cooldown (acdn) allows 0 to 3600; [BeyondLimits] goes past it" },
            Messages(Bolt("""Set("acdn", 2, 3900f);""")));
    }

    [Fact]
    public void set_with_levels_checks_each_level()
    {
        Assert.Equal(new[] { "MW0009: 3900f" }, Found(Bolt("""Set("acdn", Levels(1f, 3900f));""")));
    }

    [Fact]
    public void a_set_of_an_unknown_id_or_with_a_computed_level_gives_nothing()
    {
        Assert.Empty(Found("""
            public class Odd : Moonwell.Objects.Human.Abilities.StormBolt
            {
                private static string Id() => "uhpm";

                private static int Level() => 2;

                public Odd()
                {
                    Set("zzzz", 600000);
                    Set(Id(), 600000);
                    Set("acdn", Level(), 3900f);
                }
            }
            """));
    }

    [Fact]
    public void a_value_inside_its_limits_gives_nothing()
    {
        Assert.Empty(Found(
            Bolt("Cooldown = Levels(0f, 3600f); Set(\"acdn\", 2, 1f);")
            + Class("Big", "Moonwell.Objects.Human.Units.Footman", "HitPoints = 500000; Set(\"uhpm\", 1);")));
    }

    [Fact]
    public void a_computed_value_gives_nothing()
    {
        Assert.Empty(Found("""
            public class Computed : Moonwell.Objects.Human.Abilities.StormBolt
            {
                public Computed(float factor)
                {
                    Cooldown = PerLevel(level => 1300f * level);
                    Cooldown = Levels(factor, 2f);
                    Set("acdn", 2, factor * 9000f);
                }
            }

            public class Sized : Moonwell.Objects.Human.Units.Footman
            {
                private static int Hit() => 600000;

                public Sized(int hp, string name)
                {
                    HitPoints = hp;
                    HitPoints = Hit();
                    HitPoints = hp * 600000;
                    Name = name;
                    Set("uhpm", Hit());
                    Set("uhpm", hp);
                }
            }
            """));
    }

    [Fact]
    public void a_constant_expression_is_a_constant()
    {
        Assert.Equal(new[] { "MW0009: Big", "MW0009: 2 * Big" }, Found("""
            public class Sized : Moonwell.Objects.Human.Units.Footman
            {
                private const int Big = 600000;

                public Sized()
                {
                    HitPoints = Big;
                    HitPoints = 2 * Big;
                }
            }
            """));
    }

    [Fact]
    public void a_float_is_compared_with_the_limits_as_a_float()
    {
        Assert.Equal(
            new[] { "Tree.Scale is 0.0099999; Default Scale (ddes) allows 0.01 to 100; [BeyondLimits] goes past it" },
            Messages(Class("Tree", "Moonwell.Objects.Doodads.AlchemySet", "Scale = 0.01f; Scale = 100f; Scale = 0.0099999f;")));
    }

    [Fact]
    public void the_float_above_the_upper_limit_is_outside_it()
    {
        Assert.Equal(
            new[] { "Tree.Scale is 100.00001; Default Scale (ddes) allows 0.01 to 100; [BeyondLimits] goes past it" },
            Messages(Class("Tree", "Moonwell.Objects.Doodads.AlchemySet", "Scale = 100.00001f;")));
    }

    [Fact]
    public void a_property_named_in_beyond_limits_gives_nothing()
    {
        Assert.Empty(Found(Footman("HitPoints = 600000; Set(\"uhpm\", 700000);", "[BeyondLimits(nameof(HitPoints))]")));
    }

    [Fact]
    public void beyond_limits_is_inherited_from_a_base_class()
    {
        Assert.Empty(Found("""
            [BeyondLimits(nameof(HitPoints))]
            public abstract class Colossus : Moonwell.Objects.Human.Units.Footman
            {
            }

            public class Giant : Colossus
            {
                public Giant()
                {
                    HitPoints = 600000;
                }
            }
            """));
    }

    [Fact]
    public void beyond_limits_lifts_only_the_properties_it_names()
    {
        Assert.Equal(
            new[] { "Giant.Armor is 5000; Defense Base (udef) allows 0 to 1000; [BeyondLimits] goes past it" },
            Messages(Footman("HitPoints = 600000; Armor = 5000;", "[BeyondLimits(nameof(HitPoints))]")));
    }

    [Fact]
    public void beyond_limits_with_several_attributes_and_names_lifts_all()
    {
        Assert.Empty(Found(Footman(
            "HitPoints = 600000; Armor = 5000; Speed = 900;",
            "[BeyondLimits(nameof(HitPoints), nameof(Armor))]\n[BeyondLimits(nameof(Speed))]")));
    }

    [Fact]
    public void beyond_limits_never_lifts_a_negative()
    {
        Assert.Equal(
            new[] { "Giant.HitPoints is -5; Hit Points Maximum (Base) (uhpm) allows 0 or more, even with [BeyondLimits]" },
            Messages(Footman("HitPoints = -5;", "[BeyondLimits(nameof(HitPoints))]")));
    }

    [Fact]
    public void a_negative_that_is_not_lifted_names_the_range_without_the_hint()
    {
        Assert.Equal(
            new[] { "Giant.HitPoints is -5; Hit Points Maximum (Base) (uhpm) allows 1 to 500000" },
            Messages(Footman("HitPoints = -5;")));
    }

    [Fact]
    public void an_empty_or_null_name_is_rejected_and_never_lifted()
    {
        Assert.Equal(
            new[]
            {
                "Giant.Name is \"\"; Name (unam) allows any text that is not empty",
                "Giant.Name is null; Name (unam) allows any text that is not empty",
                "Giant.Name is \"\"; Name (unam) allows any text that is not empty",
            },
            Messages(Footman("""
                Name = "";
                Name = null;
                Set("unam", "");
                """, "[BeyondLimits(nameof(Name))]")));
    }

    [Fact]
    public void a_name_with_text_gives_nothing()
    {
        Assert.Empty(Found(Footman("""Name = "Giant";""")));
    }

    [Fact]
    public void a_text_that_is_too_long_is_rejected_unless_lifted()
    {
        var suffix = new string('x', 51);
        var text = $"Mage.NameEditorSuffix is \"{suffix}\" (51 characters); Name - Editor Suffix (unsf) allows at most 50 characters; [BeyondLimits] goes past it";
        Assert.Equal(new[] { text }, Messages(Archmage($"NameEditorSuffix = \"{suffix}\";")));
        Assert.Empty(Found(Archmage($"NameEditorSuffix = \"{suffix}\";", "[BeyondLimits(nameof(NameEditorSuffix))]")));
        Assert.Empty(Found(Archmage($"NameEditorSuffix = \"{new string('x', 50)}\";")));
    }

    [Fact]
    public void a_text_list_with_too_many_entries_is_rejected_unless_lifted()
    {
        var names = string.Join(",", Enumerable.Range(1, 33).Select(i => "N" + i));
        var text = $"Mage.ProperNames is \"{names}\" (33 entries); Proper Names (upro) allows at most 32 entries; [BeyondLimits] goes past it";
        Assert.Equal(new[] { text }, Messages(Archmage($"ProperNames = \"{names}\";")));
        Assert.Empty(Found(Archmage($"ProperNames = \"{names}\";", "[BeyondLimits(nameof(ProperNames))]")));
        Assert.Empty(Found(Archmage("ProperNames = \"\"; ProperNames = \"A,B\";")));
    }

    [Fact]
    public void a_number_list_held_as_text_checks_each_number()
    {
        Assert.Equal(
            new[] { "Bolt.RequirementsLevels is \"1,200\"; Requirements - Levels (arqa) allows at most 100 for each number; [BeyondLimits] goes past it" },
            Messages(Bolt("RequirementsLevels = \"1,200\";")));
        Assert.Empty(Found(Bolt("RequirementsLevels = \"1,100\"; RequirementsLevels = \"x\";")));
    }

    [Fact]
    public void a_choice_that_is_no_member_is_rejected_and_never_lifted()
    {
        Assert.Equal(
            new[]
            {
                "Giant.AttackType is 42; Attack 1 - Attack Type (ua1t) allows only the values of AttackType",
                "Giant.DeathType is 9; Death Type (udea) allows only the values of DeathType",
            },
            Messages(Footman("""
                AttackType = (AttackType)42;
                DeathType = (DeathType)9;
                AttackType = AttackType.Normal;
                """, "[BeyondLimits(nameof(AttackType), nameof(DeathType))]")));
    }

    [Fact]
    public void a_flags_choice_may_combine_members_but_not_use_a_bit_no_member_has()
    {
        Assert.Equal(
            new[] { "Mage.PlacementRequires is 1024; Placement Requires (upap) allows only the values of PathingListPrevent and their combinations" },
            Messages(Archmage("""
                PlacementRequires = PathingListPrevent.Unbuildable | PathingListPrevent.Unflyable;
                PlacementRequires = (PathingListPrevent)(1 << 10);
                """)));
    }

    [Fact]
    public void a_class_nested_in_another_is_named_with_its_outer_class()
    {
        Assert.Equal(
            new[] { "Heroes.Giant.HitPoints is 600000; Hit Points Maximum (Base) (uhpm) allows 1 to 500000; [BeyondLimits] goes past it" },
            Messages("""
                public static class Heroes
                {
                    public class Giant : Moonwell.Objects.Human.Units.Footman
                    {
                        public Giant()
                        {
                            HitPoints = 600000;
                        }
                    }
                }
                """));
    }

    [Fact]
    public void the_class_that_holds_the_assignment_is_named_and_lifted()
    {
        Assert.Equal(
            new[] { "Base.HitPoints is 600000; Hit Points Maximum (Base) (uhpm) allows 1 to 500000; [BeyondLimits] goes past it" },
            Messages("""
                public abstract class Base : Moonwell.Objects.Human.Units.Footman
                {
                    protected Base()
                    {
                        HitPoints = 600000;
                    }
                }

                [BeyondLimits(nameof(HitPoints))]
                public class Derived : Base
                {
                }
                """));
    }

    [Fact]
    public void an_assignment_in_a_lambda_or_a_method_is_checked_too()
    {
        Assert.Equal(new[] { "MW0009: 600000", "MW0009: 700000" }, Found("""
            public class Giant : Moonwell.Objects.Human.Units.Footman
            {
                public Giant()
                {
                    Action a = () => HitPoints = 600000;
                }

                private void Grow()
                {
                    HitPoints = 700000;
                }
            }
            """));
    }

    [Fact]
    public void a_beyond_limits_name_that_is_no_property_is_rejected()
    {
        Assert.Equal(
            new[] { "[BeyondLimits] on Giant names 'Hitpoints', which is no field property of Giant or its base classes" },
            Messages("""
                [BeyondLimits(nameof(HitPoints), "Hitpoints")]
                public class Giant : Moonwell.Objects.Human.Units.Footman
                {
                }
                """));
    }

    [Fact]
    public void a_beyond_limits_name_that_is_a_base_class_property_is_accepted()
    {
        Assert.Empty(Found("""
            [BeyondLimits("HitPoints", nameof(Name))]
            public class Giant : Moonwell.Objects.Human.Units.Footman
            {
            }
            """));
    }

    [Fact]
    public void a_beyond_limits_diagnostic_points_at_the_name()
    {
        Assert.Equal(new[] { "MW0009: \"Hitpoints\"" }, Found("""
            [BeyondLimits(nameof(HitPoints), "Hitpoints")]
            public class Giant : Moonwell.Objects.Human.Units.Footman
            {
            }
            """));
    }

    [Fact]
    public void a_set_that_is_not_the_objects_set_is_left_alone()
    {
        Assert.Empty(Found("""
            public class Plain
            {
                public Plain()
                {
                    Set("uhpm", 600000);
                    Set("uhpm", 2, 600000);
                }

                private static void Set(string field, object value) { }

                private static void Set(string field, int level, object value) { }
            }
            """));
    }
}
