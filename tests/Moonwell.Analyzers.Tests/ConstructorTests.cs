namespace Moonwell.Analyzers.Tests;

/// <summary>MW0012: the constructor of a declared unit, hero or item class runs at build time, where the game's live values are not there.</summary>
public class ConstructorTests
{
    [Fact]
    public void a_live_member_in_a_declared_class_constructor_is_an_error()
    {
        // The live members a declared field shares (Mana, MaxLife, Strength, Charges) are the class's declared values there.
        Assert.Equal(new[]
        {
            "MW0012: Life",
            "MW0012: Kill",
            "MW0012: X",
            "MW0012: Facing",
            "MW0012: Level",
            "MW0012: Revive",
            "MW0012: Remove",
        }, MapCode.CheckFile("""
            using Moonwell;
            using Moonwell.Objects;

            namespace Map
            {
                public class Mage : Human.Units.Footman
                {
                    public Mage()
                    {
                        Mana = 300;
                        MaxMana = 300;
                        Life = 10.0f;
                        this.MaxLife = 10.0f;
                        Scale = 1.2f;
                        Kill();
                        var x = X;
                        base.Facing = x;
                    }
                }

                public class Champion : Human.Units.Paladin
                {
                    public Champion() => Strength = 22;

                    public Champion(int stats)
                    {
                        Agility = stats;
                        Intelligence += stats;
                        Level = 2;
                        Revive(0.0f, 0.0f);
                    }
                }

                public class Potion : Items.ClawsOfAttack15
                {
                    public Potion()
                    {
                        Charges = 3;
                        Remove();
                    }
                }
            }
            """));
    }

    [Fact]
    public void the_message_says_what_to_set_instead()
    {
        Assert.Equal(
            new[]
            {
                "'Life' is the unit's value in the game: a unit starts with full life; set MaxLife",
                "'Add' starts a component in the game, which a declared class's constructor cannot do; use Components.Add(...)",
                "'Kill' calls the game, which a declared class's constructor cannot do; game work starts from a reaction or the game's start",
                "'Level' is the hero's level in the game: a hero starts at level 1; its level is not declared",
            },
            MapCode.MessagesOfFile("""
                using Moonwell;
                using Moonwell.Objects;

                namespace Map
                {
                    public sealed class Counter : UnitComponent
                    {
                    }

                    public class Mage : Human.Units.Footman
                    {
                        public Mage()
                        {
                            Life = 10.0f;
                            Add(new Counter());
                            Kill();
                        }
                    }

                    public class Champion : Human.Units.Paladin
                    {
                        public Champion()
                        {
                            var level = Level;
                        }
                    }
                }
                """));
    }

    [Fact]
    public void declared_and_shared_fields_components_later_code_and_classes_the_game_makes_are_accepted()
    {
        Assert.Empty(MapCode.CheckFile("""
            using System;
            using Moonwell;
            using Moonwell.Objects;

            namespace Map
            {
                public sealed class Counter : UnitComponent
                {
                }

                public class Mage : Human.Units.Footman
                {
                    private readonly Func<float> mana;

                    public Mage()
                    {
                        Name = "Mage";
                        MaxMana = 300;
                        Mana = MaxMana;
                        MaxLife = MaxLife + 200;
                        this.Scale = 1.2f;
                        base.FlyHeight = 50.0f;
                        Level = 2;
                        GoldCost = 100;
                        Components.Add(new Counter());
                        mana = () => Mana;
                        var named = nameof(Mana);
                        Local();

                        void Local()
                        {
                            Kill();
                        }
                    }

                    public Mage(Mage other)
                    {
                        MaxMana = other.MaxMana;
                    }

                    protected override void OnCreated()
                    {
                        Mana = 300;
                        Scale = 1.2f;
                    }
                }

                public class Champion : Human.Units.Paladin
                {
                    public Champion()
                    {
                        Strength = 22;
                        Agility = Strength;
                        Intelligence += 2;
                        MaxLife = 800;
                    }
                }

                public class Potion : Items.ClawsOfAttack15
                {
                    public Potion()
                    {
                        Charges = 3;
                    }
                }

                // A class the game makes when a unit enters: its constructor runs in the game.
                public class Walker : Unit
                {
                    public Walker()
                    {
                        var life = Life;
                    }
                }
            }
            """));
    }

    [Fact]
    public void a_heros_level_set_by_its_field_is_the_same_error()
    {
        Assert.Equal(
            new[]
            {
                "'Set(\"ulev\")' is the hero's level in the game: a hero starts at level 1; its level is not declared",
            },
            MapCode.MessagesOfFile("""
                using Moonwell;
                using Moonwell.Objects;

                namespace Map
                {
                    public class Champion : Human.Units.Paladin
                    {
                        public Champion()
                        {
                            Set("ulev", 5);
                            Set("uhpm", 900);
                        }
                    }

                    public class Mage : Human.Units.Footman
                    {
                        public Mage()
                        {
                            Set("ulev", 5);
                        }
                    }
                }
                """));
    }

    [Fact]
    public void a_units_level_set_in_a_component_is_mw0015()
    {
        var source = """
            using Moonwell;
            using Moonwell.Objects;

            namespace Map
            {
                public sealed class Promote : UnitComponent
                {
                    protected override void OnAdded()
                    {
                        Unit.Level = 3;
                        Unit.Level++;
                        var read = Unit.Level;
                    }
                }

                public class Walker : Unit
                {
                    public Walker()
                    {
                        Level = 2;
                    }
                }
            }
            """;

        Assert.Equal(new[] { "MW0015: Unit.Level", "MW0015: Unit.Level", "MW0015: Level" }, MapCode.CheckFile(source));
        Assert.Equal("a unit's level is set only in its class's constructor", MapCode.MessagesOfFile(source)[0]);
    }

    [Fact]
    public void a_units_level_set_in_a_declared_class_constructor_and_its_lambdas_is_accepted()
    {
        Assert.Empty(MapCode.CheckFile("""
            using System;
            using Moonwell;
            using Moonwell.Objects;

            namespace Map
            {
                public abstract class Veteran : Human.Units.Footman
                {
                    protected Veteran()
                    {
                        Level = 3;
                    }
                }

                public class Elite<T> : Human.Units.Footman
                {
                    public Elite()
                    {
                        this.Level = 4;
                    }
                }

                public class Captain : Veteran
                {
                    public Captain()
                    {
                        base.Level = 5;
                        Action later = () => Level = 6;
                        Wrap();

                        void Wrap()
                        {
                            Level = 7;
                        }
                    }
                }
            }
            """));
    }
}
