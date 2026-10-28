namespace Moonwell.Analyzers.Tests;

/// <summary>MW0041: a reaction of a Moonwell class written without override, or a helper with a reaction's name and other parameters, which Moonwell never calls.</summary>
public class CoreReactionTests
{
    [Fact]
    public void a_reaction_without_override_or_with_new_is_an_error()
    {
        Assert.Equal(new[] { "MW0041: OnDamaged", "MW0041: OnKill", "MW0041: OnCreated", "MW0041: OnGameStart", "MW0041: OnTick" }, MapCode.CheckFile("""
            using Moonwell;

            namespace Map
            {
                public class Guard : Unit
                {
                    public void OnDamaged(Damage damage) { }

                    public new void OnKill(Unit killedUnit) { }

                    protected void OnCreated() { }
                }

                public class Rules : GameComponent
                {
                    public void OnGameStart() { }
                }

                public class Burn : Buff
                {
                    public void OnTick() { }
                }
            }
            """));
    }

    [Fact]
    public void a_missile_reaction_without_override_is_an_error()
    {
        Assert.Equal(new[] { "MW0041: OnImpact", "MW0041: OnRest", "MW0041: OnExpire" }, MapCode.CheckFile("""
            using Moonwell;

            namespace Map
            {
                public class Bolt : Missile
                {
                    public void OnImpact(Impact impact) { }

                    public new void OnRest() { }

                    protected void OnExpire() { }
                }
            }
            """));
    }

    [Fact]
    public void a_helper_named_like_a_reaction_with_other_parameters_is_an_error()
    {
        Assert.Equal(new[] { "MW0041: OnDeath", "MW0041: OnUnitDied" }, MapCode.CheckFile("""
            using Moonwell;

            namespace Map
            {
                public class Guard : Unit
                {
                    public override void OnDeath(Unit? killerUnit) { }

                    public void OnDeath(string why) { }
                }

                public class Rules : GameComponent
                {
                    public void OnUnitDied(Unit dyingUnit) { }
                }
            }
            """));
    }

    [Fact]
    public void right_reactions_and_other_helpers_pass()
    {
        Assert.Empty(MapCode.CheckFile("""
            using Moonwell;

            namespace Map
            {
                public class Guard : Unit
                {
                    public override void OnDamaged(Damage damage) { }

                    protected override void OnCreated() { }

                    public void OnHurt() { }

                    public void Refresh() { }
                }

                public class Elite : Guard
                {
                    public override void OnDamaged(Damage damage) { }
                }

                public class Rules : GameComponent
                {
                    public override void OnGameStart() { }

                    public void OnWave(int number) { }
                }

                public class Base : Unit
                {
                    public virtual void OnSpecial() { }
                }

                public class Derived : Base
                {
                    public void OnSpecial(int times) { }
                }
            }
            """));
    }

    [Fact]
    public void a_reaction_hidden_below_a_map_override_is_an_error()
    {
        Assert.Equal(new[] { "MW0041: OnDamaged" }, MapCode.CheckFile("""
            using Moonwell;

            namespace Map
            {
                public class Guard : Unit
                {
                    public override void OnDamaged(Damage damage) { }
                }

                public class Elite : Guard
                {
                    public void OnDamaged(Damage damage) { }
                }
            }
            """));
    }

    [Fact]
    public void the_message_says_what_to_write()
    {
        Assert.Equal(new[]
        {
            "'OnDamaged' never runs: it hides Unit.OnDamaged instead of overriding it; write public override void OnDamaged(Damage damage)",
            "'OnCreated' never runs: it hides Unit.OnCreated instead of overriding it; write protected override void OnCreated()",
            "'OnDeath' is not the reaction Unit.OnDeath(Unit? killerUnit), which takes other parameters, so it never runs as one, and on .NET it makes adding the class fail; give the helper another name",
        }, MapCode.MessagesOfFile("""
            using Moonwell;

            namespace Map
            {
                public class Guard : Unit
                {
                    public void OnDamaged(Damage damage) { }

                    protected void OnCreated() { }

                    public void OnDeath(string why) { }
                }
            }
            """));
    }
}
