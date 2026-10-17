namespace Moonwell.Testing
{
    /// <summary>A game component that logs the damage and the death of units in the map.</summary>
    public class GameWatch : GameComponent
    {
        /// <summary>Logs the damage.</summary>
        public override void OnUnitDamaged(Damage damage)
        {
            TestLog.Lines.Add("game damaged");
        }

        /// <summary>Logs the death.</summary>
        public override void OnUnitDied(Unit dying, Unit? killer)
        {
            TestLog.Lines.Add("game died");
        }
    }
}
