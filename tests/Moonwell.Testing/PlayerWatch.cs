namespace Moonwell.Testing
{
    /// <summary>A player component that logs the damage and the death of units in the map, under its label.</summary>
    public class PlayerWatch : PlayerComponent
    {
        /// <summary>The name written at the start of each line.</summary>
        private readonly string label;

        /// <summary>Makes a watch that logs under a label.</summary>
        public PlayerWatch(string label)
        {
            this.label = label;
        }

        /// <summary>Logs the damage under the label.</summary>
        public override void OnUnitDamaged(Damage damage)
        {
            TestLog.Lines.Add(label + " damaged");
        }

        /// <summary>Logs the death under the label.</summary>
        public override void OnUnitDied(Unit dying, Unit? killer)
        {
            TestLog.Lines.Add(label + " died");
        }

        /// <summary>Logs the cast under the label.</summary>
        public override void OnUnitCast(Cast cast)
        {
            TestLog.Lines.Add(label + " cast");
        }
    }
}
