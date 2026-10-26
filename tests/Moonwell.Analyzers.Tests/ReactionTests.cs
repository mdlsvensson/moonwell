namespace Moonwell.Analyzers.Tests;

/// <summary>MW0036: a damage reaction the package would never call, which the compiler accepts because the interfaces have default bodies.</summary>
public class ReactionTests
{
    [Fact]
    public void a_reaction_that_is_not_public_or_takes_damage_is_an_error()
    {
        Assert.Equal(new[] { "MW0036: OnTaken", "MW0036: OnTaken", "MW0036: OnHurt" }, MapCode.CheckFile("""
            using Moonwell;

            namespace Map
            {
                public class Hidden : Unit, IDamageTarget
                {
                    void OnTaken(Hit hit) { }
                }

                public class Wrong : Unit, IDamageTarget
                {
                    public void OnTaken(Damage damage) { }
                }

                public class Explicit : Unit, IDamageTarget
                {
                    void IDamageTarget.OnHurt(Hit hit) { }
                }
            }
            """));
    }

    [Fact]
    public void a_near_miss_name_is_an_error()
    {
        Assert.Equal(new[] { "MW0036: OnTakn" }, MapCode.CheckFile("""
            using Moonwell;

            namespace Map
            {
                public class Typo : Unit, IDamageTarget
                {
                    public void OnTakn(Hit hit) { }
                }
            }
            """));
    }

    [Fact]
    public void a_reaction_without_its_interface_or_with_an_old_name_is_an_error()
    {
        Assert.Equal(new[] { "MW0036: OnDealt", "MW0036: OnLanded", "MW0036: OnLethal" }, MapCode.CheckFile("""
            using Moonwell;

            namespace Map
            {
                public class Forgot : Unit
                {
                    public void OnDealt(Hit hit) { }
                }

                public class OldWatch : GameComponent, IDamageWatch
                {
                    public void OnLanded(Hit hit) { }
                }

                public class OldTarget : Unit, IDamageTarget
                {
                    public void OnLethal(Hit hit) { }
                }
            }
            """));
    }

    [Fact]
    public void an_interface_on_a_class_the_package_never_calls_is_an_error()
    {
        Assert.Equal(new[] { "MW0036: IDamageSource", "MW0036: IDamageWatch" }, MapCode.CheckFile("""
            using Moonwell;

            namespace Map
            {
                public class Rules : GameComponent, IDamageSource
                {
                    public void OnDealt(Hit hit) { }
                }

                public class Mark : UnitComponent, IDamageWatch
                {
                    public void OnUnitTaken(Hit hit) { }
                }
            }
            """));
    }

    [Fact]
    public void the_message_says_what_to_write()
    {
        Assert.Equal(new[]
        {
            "'OnTakn' is not a reaction of IDamageTarget, so it never runs; did you mean OnTaken?",
            "'OnLanded' never runs: IDamageWatch's reaction of that stage is OnUnitHurt since Moonwell 0.3",
        }, MapCode.MessagesOfFile("""
            using Moonwell;

            namespace Map
            {
                public class Typo : Unit, IDamageTarget
                {
                    public void OnTakn(Hit hit) { }
                }

                public class OldWatch : GameComponent, IDamageWatch
                {
                    public void OnLanded(Hit hit) { }
                }
            }
            """));
    }

    [Fact]
    public void helpers_named_near_a_reaction_pass()
    {
        Assert.Empty(MapCode.CheckFile("""
            using Moonwell;

            namespace Map
            {
                public class Guard : Unit, IDamageTarget
                {
                    public void OnTaken(Hit hit) { OnHit(hit); }

                    private void OnHit(Hit hit) { }

                    public void OnHeal(Hit hit) { }
                }

                public class Marker : UnitComponent
                {
                    public void OnHit(Hit hit) { }
                }
            }
            """));
    }

    [Fact]
    public void an_abstract_base_with_reactions_for_its_subclasses_passes()
    {
        Assert.Empty(MapCode.CheckFile("""
            using Moonwell;

            namespace Map
            {
                public abstract class Base : Unit
                {
                    public void OnTaken(Hit hit) { }
                }

                public class Derived : Base, IDamageTarget
                {
                }
            }
            """));
    }

    [Fact]
    public void a_reaction_in_a_class_that_inherits_the_interface_says_to_list_it()
    {
        Assert.Equal(new[]
        {
            "'OnHurt' is not IDamageTarget's reaction here, as Arthas gets IDamageTarget from a base class: list IDamageTarget among Arthas's own bases too",
        }, MapCode.MessagesOfFile("""
            using Moonwell;

            namespace Map
            {
                public abstract class Champion : Unit, IDamageTarget
                {
                }

                public class Arthas : Champion
                {
                    public void OnHurt(Hit hit) { }
                }
            }
            """));
    }

    [Fact]
    public void right_reactions_and_helpers_pass()
    {
        Assert.Empty(MapCode.CheckFile("""
            using Moonwell;

            namespace Map
            {
                public class Shield : Buff, IDamageTarget
                {
                    public void OnTaken(Hit hit) { hit.Amount *= 0.5f; }

                    public void OnHurtSound() { }
                }

                public class Blademaster : Unit, IDamageSource, IDamageTarget
                {
                    public void OnDealing(Hit hit) { }

                    public void OnBurst(Burst burst) { }

                    public void OnHurt(Hit hit) { }

                    public override void OnDamaged(Damage damage) { }
                }

                public class Watch : GameComponent, IDamageWatch
                {
                    public void OnUnitTaking(Hit hit) { }

                    public void OnUnitBurst(Burst burst) { }
                }

                public class Plain : Unit
                {
                    public void OnHurt() { }
                }
            }
            """));
    }
}
