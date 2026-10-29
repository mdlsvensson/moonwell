using Moonwell;

namespace PhysicsFixture
{
    /// <summary>A bolt that flies straight and burns the first enemy unit it meets.</summary>
    public sealed class Firebolt : Missile
    {
        public Firebolt()
        {
            Model = @"Abilities\Weapons\RedDragonBreath\RedDragonMissile.mdl";
            Speed = 900f;
            Radius = 24f;
            ImpactDamage = 75f;
            DamageKind = DamageKind.Fire;
        }

        public override bool CanHit(Actor other) => other is Unit unit && Source!.Owner.IsEnemy(unit.Owner);
    }
}
