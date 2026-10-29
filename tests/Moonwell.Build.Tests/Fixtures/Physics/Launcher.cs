using Moonwell;
using Moonwell.Objects;

namespace PhysicsFixture
{
    /// <summary>Makes two footmen at the start, and a moment later fires a firebolt from one at the other and knocks it back.</summary>
    public sealed class Launcher : GameComponent
    {
        public override void OnGameStart()
        {
            var caster = Unit.Create<Human.Units.Footman>(Player.One, 0f, 0f);
            var target = Unit.Create<Human.Units.Footman>(Player.Two, 400f, 0f);
            After(0.1f, () =>
            {
                caster.Fire(new Firebolt(), target);
                target.Knockback(caster.AngleTo(target), 300f, 0.5f);
            });
        }
    }
}
