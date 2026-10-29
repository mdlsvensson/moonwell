namespace Moonwell.Analyzers.Tests;

/// <summary>MW0043: a player given where a scope goes.</summary>
public class ScopeTests
{
    private const string Things = """
        public sealed class Well : Gizmo { }
        public sealed class Bolt : Missile { }
        """;

    [Fact]
    public void a_player_where_a_scope_goes_is_rejected()
    {
        var found = MapCode.Check("""
            var unit = Unit.Of(u)!;
            Gizmo.Create(new Well(), 0f, 0f, p);
            Missile.Create(new Bolt(), 0f, 0f, 0f, p);
            Item.Create(0, 0f, 0f, p);
            unit.Fire(new Bolt(), 0f, 0f, unit.Owner);
            unit.Interact(new Aura(), scope: p);
            p.After(1f, () => { });
            """, Things + """
            public sealed class Aura : Interaction<Unit, Unit> { }
            """);

        Assert.Equal(new[] { "MW0043: p", "MW0043: p", "MW0043: p", "MW0043: unit.Owner", "MW0043: p", "MW0043: p" }, found);
    }

    [Fact]
    public void a_player_as_owner_and_a_player_held_as_a_scope_are_accepted()
    {
        var found = MapCode.Check("""
            Gizmo.Create(new Well(), 0f, 0f, Game.Instance, p);
            Unit.Create(0, p, 0f, 0f, Game.Instance);
            Scope held = p;
            held.After(1f, () => { });
            Game.Instance.Every(1f, () => { });
            """, Things);

        Assert.Empty(found);
    }

    [Fact]
    public void a_player_components_player_is_rejected_and_the_component_is_not()
    {
        var found = MapCode.CheckFile("""
            using Moonwell;

            namespace Map
            {
                public sealed class Tracker : PlayerComponent
                {
                    public void Begin()
                    {
                        Player.After(1f, () => { });
                        After(1f, () => { });
                    }
                }
            }
            """);

        Assert.Equal(new[] { "MW0043: Player" }, found);
    }

    [Fact]
    public void the_message_says_what_to_write_instead()
    {
        Assert.Equal(
            "'p' is a player, which lasts the whole game, so it is no scope to make things in: write Game.Instance for what should last the game, or pass the player as the owner where the call takes one",
            MapCode.Message("Gizmo.Create(new Well(), 0f, 0f, p);", Things));
    }
}
