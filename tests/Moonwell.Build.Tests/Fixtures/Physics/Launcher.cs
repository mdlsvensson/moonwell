using Moonwell;
using Moonwell.Objects;

namespace PhysicsFixture
{
    /// <summary>Makes two footmen at the start, and a moment later fires a firebolt from one at the other and knocks it back.</summary>
    public sealed class Launcher : GameComponent
    {
        public override void OnGameStart()
        {
            var caster = Unit.Create<Human.Units.Footman>(Player.One, 0f, 0f, Game.Instance);
            var target = Unit.Create<Human.Units.Footman>(Player.Two, 400f, 0f, Game.Instance);
            After(0.1f, () =>
            {
                caster.Fire(new Firebolt(), target, Game.Instance);
                target.Knockback(caster.AngleTo(target), 300f, 0.5f);
            });
        }
    }
}
