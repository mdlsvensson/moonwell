namespace Moonwell
{
    /// <summary>A component on a missile. Its reactions run after the missile's own, in the order the components were added; a missile ends its components when it ends.</summary>
    public abstract class MissileComponent : GizmoComponent
    {
        /// <summary>The missile this component is on.</summary>
        [LocalSafe]
        public Missile Missile => (Missile)Actor;

        /// <summary>Runs at each contact of the missile, after the missile's own OnImpact, with the missile's owner as the current owner.</summary>
        public virtual void OnImpact(Impact impact)
        {
        }

        /// <summary>Runs once the missile lies still on the ground, after the missile's own OnRest, with the missile as the current owner.</summary>
        public virtual void OnRest()
        {
        }
    }
}
