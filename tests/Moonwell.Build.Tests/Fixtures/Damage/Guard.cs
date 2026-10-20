using Moonwell;
using Moonwell.Objects;

namespace DamageFixture
{
    /// <summary>A footman that takes half of every hit after armor.</summary>
    [ObjectId("h001")]
    public sealed class Guard : Human.Units.Footman, IDamageTarget
    {
        public Guard()
        {
            Name = "Guard";
        }

        public void OnTaken(Hit hit)
        {
            hit.Amount = hit.Amount / 2f;
        }
    }

    /// <summary>Makes two guards at the start and has one hit the other with fire.</summary>
    public sealed class Opening : GameComponent
    {
        public override void OnGameStart()
        {
            var first = Unit.Create<Guard>(Player.One, 0f, 0f);
            var second = Unit.Create<Guard>(Player.Two, 100f, 0f);
            first.Damage(second, 10f, DamageKind.Fire, tag: "opening");
        }
    }
}
