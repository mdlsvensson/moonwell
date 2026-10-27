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

    [Fact]
    public void a_mouse_reaction_the_input_package_never_calls_is_an_error()
    {
        Assert.Equal(new[]
        {
            "MW0036: IMouseWatch",
            "MW0036: OnMousePressed",
            "MW0036: OnMousePressed",
            "MW0036: OnMouseReleased",
            "MW0036: OnMouseMove",
            "MW0036: OnMouseMoved",
        }, MapCode.CheckFile("""
            using Moonwell;

            namespace Map
            {
                public class Aim : UnitComponent, IMouseWatch
                {
                    public void OnMousePressed(Mouse mouse, MouseButton button) { }
                }

                public class Explicit : PlayerComponent, IMouseWatch
                {
                    void IMouseWatch.OnMousePressed(Mouse mouse, MouseButton button) { }
                }

                public class NoButton : PlayerComponent, IMouseWatch
                {
                    public void OnMousePressed(Mouse mouse) { }

                    void OnMouseReleased(Mouse mouse, MouseButton button) { }
                }

                public class Typo : GameComponent, IMouseWatch
                {
                    public void OnMouseMove(Mouse mouse) { }
                }

                public class Forgot : GameComponent
                {
                    public void OnMouseMoved(Mouse mouse) { }
                }
            }
            """));
    }

    [Fact]
    public void a_mouse_reaction_message_says_what_to_write()
    {
        Assert.Equal(new[]
        {
            "'IMouseWatch' on this class never runs: the input package calls IMouseWatch only on the game's and players' components",
            "'OnMousePressed' runs in the game's Lua but not on .NET, where the input package calls only public methods; write it as public void OnMousePressed(...)",
            "'OnMouseReleased' is not called on .NET, while the game's Lua calls it by its name, so the two disagree: a reaction of IMouseWatch is public, not static, and takes (Mouse, MouseButton)",
        }, MapCode.MessagesOfFile("""
            using Moonwell;

            namespace Map
            {
                public class Aim : UnitComponent, IMouseWatch
                {
                }

                public class Explicit : PlayerComponent, IMouseWatch
                {
                    void IMouseWatch.OnMousePressed(Mouse mouse, MouseButton button) { }
                }

                public class NoButton : PlayerComponent, IMouseWatch
                {
                    public void OnMouseReleased(Mouse mouse) { }
                }
            }
            """));
    }

    [Fact]
    public void right_mouse_reactions_and_helpers_pass()
    {
        Assert.Empty(MapCode.CheckFile("""
            using Moonwell;

            namespace Map
            {
                public sealed class Aim : PlayerComponent, IMouseWatch
                {
                    public void OnMousePressed(Mouse mouse, MouseButton button) { }

                    public void OnMouseMoved(Mouse mouse) { }

                    public void Aiming(Mouse mouse) { }

                    private void OnAim(Mouse mouse) { }
                }

                public sealed class Everyone : GameComponent, IMouseWatch
                {
                    public void OnMouseReleased(Mouse mouse, MouseButton button) { }
                }
            }
            """));
    }

    [Fact]
    public void push_target_typo_is_MW0036()
    {
        Assert.Equal(new[] { "MW0036: OnSlamed" }, MapCode.CheckFile("""
            using Moonwell;

            namespace Map
            {
                public class Typo : Unit, IPushTarget
                {
                    public void OnSlamed(Slam slam) { }
                }
            }
            """));
    }

    [Fact]
    public void struck_without_interface_is_MW0036()
    {
        Assert.Equal(new[] { "MW0036: OnStruck" }, MapCode.CheckFile("""
            using Moonwell;

            namespace Map
            {
                public class Dazed : Buff
                {
                    public void OnStruck(Impact impact) { }
                }
            }
            """));
    }

    [Fact]
    public void the_other_physics_slips_are_MW0036_with_messages()
    {
        Assert.Equal(new[]
        {
            "'OnLanded' never runs: IPushTarget's reaction of that stage is OnGrounded",
            "'OnPushd' is not a reaction of IPushTarget, so it never runs; did you mean OnPushed?",
            "'OnGrounded' never runs: a reaction of IPushTarget is public, not static, and takes a float",
            "'OnStruck' never runs: it is a reaction of IMissileTarget, which this class does not implement",
            "'IPushTarget' on this class never runs: the physics package calls IPushTarget only on a unit's class and its components (buffs too)",
        }, MapCode.MessagesOfFile("""
            using Moonwell;

            namespace Map
            {
                public class Landed : Unit, IPushTarget
                {
                    public void OnLanded(float speed) { }
                }

                public class Typo : Unit, IPushTarget
                {
                    public void OnPushd(Unit? pusher) { }
                }

                public class Wrong : Unit, IPushTarget
                {
                    public void OnGrounded(int speed) { }
                }

                public class Forgot : Unit
                {
                    public void OnStruck(Impact impact) { }
                }

                public class Rules : GameComponent, IPushTarget
                {
                }
            }
            """));
    }

    [Fact]
    public void a_reaction_that_takes_a_nullable_unit_is_named_without_its_mark()
    {
        Assert.Equal(new[]
        {
            "'OnPushed' never runs: a reaction of IPushTarget is public, not static, and takes a Unit",
        }, MapCode.MessagesOfFile("""
            using Moonwell;

            namespace Map
            {
                public class Wrong : Unit, IPushTarget
                {
                    public void OnPushed(string pusher) { }
                }
            }
            """));
    }

    [Fact]
    public void written_right_physics_reactions_are_left_alone()
    {
        Assert.Empty(MapCode.CheckFile("""
            using Moonwell;

            namespace Map
            {
                public class Heavy : Unit, IPushTarget, IMissileTarget, IDamageSource
                {
                    public void OnPushed(Unit? pusher) { }
                    public void OnSlammed(Slam slam) { }
                    public void OnGrounded(float speed) { }
                    public void OnStruck(Impact impact) { }
                    public void OnLanded(Hit hit) { }
                    public void Helper(Impact impact) { }
                }
            }
            """));
    }
}
