namespace Moonwell.Analyzers.Tests;

/// <summary>MW0013 and MW0014: a declared object's id, checked in the editor as the build checks it.</summary>
public class IdTests
{
    [Fact]
    public void a_declared_class_without_an_id_is_an_error()
    {
        Assert.Equal(new[]
        {
            "MW0013: Forge",
            "MW0013: Claws",
            "MW0013: Burn",
        }, MapCode.CheckIds("""
            using Moonwell.Objects;

            namespace Map
            {
                public sealed class Forge : Human.Units.Blacksmith
                {
                }

                public sealed class Claws : Items.ClawsOfAttack15
                {
                }

                public sealed class Burn : Human.Buffs.FlameStrike
                {
                }
            }
            """));
    }

    [Fact]
    public void a_class_with_an_id_a_replacing_class_and_an_abstract_class_need_none()
    {
        Assert.Empty(MapCode.CheckIds("""
            using Moonwell.Objects;

            namespace Map
            {
                [ObjectId("h001")]
                public sealed class Forge : Human.Units.Blacksmith
                {
                }

                [Replaces]
                public sealed class Footman : Human.Units.Footman
                {
                }

                public abstract class Base : Human.Units.Rifleman
                {
                }

                [ObjectId("h002")]
                public sealed class Sniper : Base
                {
                }

                [ObjectId("B001", Carrier = "A001")]
                public sealed class Burn : Human.Buffs.FlameStrike
                {
                }

                public sealed class Plain
                {
                }
            }
            """));
    }

    [Fact]
    public void a_buff_without_a_carrier_is_an_error()
    {
        Assert.Equal(new[]
        {
            "MW0013: ObjectId(\"B001\")",
            "MW0013: Replaces",
        }, MapCode.CheckIds("""
            using Moonwell.Objects;

            namespace Map
            {
                [ObjectId("B001")]
                public sealed class Burn : Human.Buffs.FlameStrike
                {
                }

                [Replaces]
                public sealed class Slow : Human.Buffs.Slow
                {
                }
            }
            """));
    }

    [Fact]
    public void the_message_says_what_to_add()
    {
        Assert.Equal(new[]
        {
            "Forge has no object id; add [ObjectId(\"...\")] with a unit's id, which starts with a lower-case letter; moonwell build names a free one",
            "Champion has no object id; add [ObjectId(\"...\")] with a hero's id, which starts with an upper-case letter; moonwell build names a free one",
            "Burn has no carrier id; add Carrier = \"...\" to its [ObjectId], the id of the hidden ability that shows the buff's icon; moonwell build names a free one",
        }, MapCode.MessagesOfIds("""
            using Moonwell.Objects;

            namespace Map
            {
                public sealed class Forge : Human.Units.Blacksmith
                {
                }

                public sealed class Champion : Human.Units.Paladin
                {
                }

                [ObjectId("B001")]
                public sealed class Burn : Human.Buffs.FlameStrike
                {
                }
            }
            """));
    }

    [Fact]
    public void an_id_of_the_wrong_shape_is_an_error()
    {
        Assert.Equal(new[]
        {
            "MW0014: \"H001\"",
            "MW0014: \"h002\"",
            "MW0014: \"I01\"",
            "MW0014: \"A0011\"",
        }, MapCode.CheckIds("""
            using Moonwell.Objects;

            namespace Map
            {
                [ObjectId("H001")]
                public sealed class Forge : Human.Units.Blacksmith
                {
                }

                [ObjectId("h002")]
                public sealed class Champion : Human.Units.Paladin
                {
                }

                [ObjectId("I01")]
                public sealed class Claws : Items.ClawsOfAttack15
                {
                }

                [ObjectId("B001", Carrier = "A0011")]
                public sealed class Burn : Human.Buffs.FlameStrike
                {
                }
            }
            """));
    }

    [Fact]
    public void an_id_with_a_character_outside_ascii_is_an_error()
    {
        // The object file would hold "?001", and the Lua another id still.
        Assert.Equal(new[]
        {
            "Arthas has the id \"Å001\", with a character that is not an ASCII letter, digit or sign",
            "Burn has the id \"A 01\", with a character that is not an ASCII letter, digit or sign",
        }, MapCode.MessagesOfIds("""
            using Moonwell.Objects;

            namespace Map
            {
                [ObjectId("Å001")]
                public sealed class Arthas : Human.Units.Paladin
                {
                }

                [ObjectId("B001", Carrier = "A 01")]
                public sealed class Burn : Human.Buffs.FlameStrike
                {
                }

                [ObjectId("h!~0")]
                public sealed class Signs : Human.Units.Footman
                {
                }
            }
            """));
    }

    [Fact]
    public void an_id_with_a_comma_is_an_error()
    {
        Assert.Equal(new[]
        {
            "Comma has the id \"h0,0\", with a comma, which splits a list field that names the object",
        }, MapCode.MessagesOfIds("""
            using Moonwell.Objects;

            namespace Map
            {
                [ObjectId("h0,0")]
                public sealed class Comma : Human.Units.Footman
                {
                }

                [ObjectId("h0'0")]
                public sealed class Quote : Human.Units.Footman
                {
                }
            }
            """));
    }

    [Fact]
    public void a_wrong_case_message_says_why()
    {
        Assert.Equal(new[]
        {
            "Forge is a unit, so its id H001 must not start with an upper-case letter: the game makes such a unit a hero",
            "Champion is a hero, so its id h002 must start with an upper-case letter",
            "Claws has the id \"I01\", which is not four characters",
        }, MapCode.MessagesOfIds("""
            using Moonwell.Objects;

            namespace Map
            {
                [ObjectId("H001")]
                public sealed class Forge : Human.Units.Blacksmith
                {
                }

                [ObjectId("h002")]
                public sealed class Champion : Human.Units.Paladin
                {
                }

                [ObjectId("I01")]
                public sealed class Claws : Items.ClawsOfAttack15
                {
                }
            }
            """));
    }
}
