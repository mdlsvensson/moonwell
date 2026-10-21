namespace Moonwell.Analyzers.Tests;

/// <summary>MW0012: the constructor of a declared unit, hero or item class runs at build time, where the game's live values are not there.</summary>
public class ConstructorTests
{
    [Fact]
    public void a_live_member_in_a_declared_class_constructor_is_an_error()
    {
        Assert.Equal(new[]
        {
            "MW0012: Mana",
            "MW0012: MaxMana",
            "MW0012: Life",
            "MW0012: MaxLife",
            "MW0012: Scale",
            "MW0012: Kill",
            "MW0012: X",
            "MW0012: Facing",
            "MW0012: Strength",
            "MW0012: Agility",
            "MW0012: Intelligence",
            "MW0012: Revive",
            "MW0012: Charges",
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
    public void the_message_names_the_declared_field()
    {
        Assert.Equal(
            new[]
            {
                "'Mana' is the unit's value in the game, which a declared class's constructor cannot reach; the declared field is ManaPoints",
                "'MaxMana' is the unit's value in the game, which a declared class's constructor cannot reach; the declared field is ManaPoints",
                "'Life' is the unit's value in the game, which a declared class's constructor cannot reach; the declared field is HitPoints",
                "'MaxLife' is the unit's value in the game, which a declared class's constructor cannot reach; the declared field is HitPoints",
                "'Scale' is the unit's value in the game, which a declared class's constructor cannot reach; the declared field is ScalingValue",
                "'Level' is the unit's value in the game, which a declared class's constructor cannot reach; the declared field is UnitLevel",
                "'FlyHeight' is the unit's value in the game, which a declared class's constructor cannot reach; the declared field is Height",
                "'Add' starts a component in the game, which a declared class's constructor cannot do; use Components.Add(...)",
                "'Kill' calls the game, which a declared class's constructor cannot do; game work starts from a reaction or the game's start",
                "'Strength' is the hero's value in the game, which a declared class's constructor cannot reach; the declared field is StartingStrength",
                "'Agility' is the hero's value in the game, which a declared class's constructor cannot reach; the declared field is StartingAgility",
                "'Intelligence' is the hero's value in the game, which a declared class's constructor cannot reach; the declared field is StartingIntelligence",
                "'Charges' is the item's value in the game, which a declared class's constructor cannot reach; the declared field is NumberOfCharges",
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
                            Mana = 300;
                            MaxMana = 300;
                            Life = 10.0f;
                            MaxLife = 10.0f;
                            Scale = 1.2f;
                            var level = Level;
                            FlyHeight = 100.0f;
                            Add(new Counter());
                            Kill();
                        }
                    }

                    public class Champion : Human.Units.Paladin
                    {
                        public Champion()
                        {
                            Strength = 22;
                            Agility = 1;
                            Intelligence = 2;
                        }
                    }

                    public class Potion : Items.ClawsOfAttack15
                    {
                        public Potion()
                        {
                            Charges = 3;
                        }
                    }
                }
                """));
    }

    [Fact]
    public void declared_fields_components_later_code_and_classes_the_game_makes_are_accepted()
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
                        ManaPoints = 300;
                        HitPoints = HitPoints + 200;
                        ScalingValue = 1.2f;
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
                        ManaPoints = (int)other.Mana;
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
                        StartingStrength = 22;
                    }
                }

                public class Potion : Items.ClawsOfAttack15
                {
                    public Potion()
                    {
                        NumberOfCharges = 3;
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
}
