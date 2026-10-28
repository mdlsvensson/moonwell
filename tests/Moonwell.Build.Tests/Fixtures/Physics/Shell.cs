using Moonwell;

namespace PhysicsFixture
{
    /// <summary>A mortar shell that arcs to a point and blasts what is near where it lands (the guide's example); the map never fires it.</summary>
    public sealed class Shell : Missile
    {
        public Shell()
        {
            Model = @"Abilities\Weapons\Mortar\MortarMissile.mdl";
            Speed = 700f;
            Arc = 0.35f;
            UnitContact = Contact.None;
            Components.Add(new Shadow());
        }

        public override void OnImpact(Impact impact)
        {
            foreach (var unit in Find.InRange<Unit>(impact.X, impact.Y, 200f, u => Source!.Player.IsEnemy(u.Player)))
            {
                Source!.Damage(unit, 60f, tag: this);
            }
        }
    }
}
