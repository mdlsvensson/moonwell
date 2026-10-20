namespace Moonwell.Testing
{
    /// <summary>A unit that logs every stage of the hits it deals, and core's OnDealtDamage; starts with a SourceComp.</summary>
    public class SourceUnit : Unit, IDamageSource
    {
        /// <summary>Makes a source with its component.</summary>
        public SourceUnit()
        {
            Components.Add(new SourceComp());
        }

        /// <summary>Logs the stage.</summary>
        public void OnDealing(Hit hit) => TestLog.Lines.Add("source dealing");

        /// <summary>Logs the stage.</summary>
        public void OnDealt(Hit hit) => TestLog.Lines.Add("source dealt");

        /// <summary>Logs core's reaction.</summary>
        public override void OnDealtDamage(Damage damage) => TestLog.Lines.Add("source core dealt");
    }

    /// <summary>A component that logs the stages of the hits its unit deals.</summary>
    public class SourceComp : UnitComponent, IDamageSource
    {
        /// <summary>Logs the stage.</summary>
        public void OnDealing(Hit hit) => TestLog.Lines.Add("source comp dealing");

        /// <summary>Logs the stage.</summary>
        public void OnDealt(Hit hit) => TestLog.Lines.Add("source comp dealt");
    }

    /// <summary>A unit that logs every stage of the hits it takes, and core's OnDamaged; starts with a TargetComp. Its Taking and Taken actions, when set, run after the log.</summary>
    public class TargetUnit : Unit, IDamageTarget
    {
        /// <summary>Run in OnTaking after the log; null for nothing.</summary>
        public System.Action<Hit>? Taking;

        /// <summary>Run in OnTaken after the log; null for nothing.</summary>
        public System.Action<Hit>? Taken;

        /// <summary>Makes a target with its component.</summary>
        public TargetUnit()
        {
            Components.Add(new TargetComp());
        }

        /// <summary>Logs the stage, then runs Taking.</summary>
        public void OnTaking(Hit hit)
        {
            TestLog.Lines.Add("target taking");
            if (Taking != null)
            {
                Taking(hit);
            }
        }

        /// <summary>Logs the stage, then runs Taken.</summary>
        public void OnTaken(Hit hit)
        {
            TestLog.Lines.Add("target taken");
            if (Taken != null)
            {
                Taken(hit);
            }
        }

        /// <summary>Logs core's reaction.</summary>
        public override void OnDamaged(Damage damage) => TestLog.Lines.Add("target core damaged");
    }

    /// <summary>A component that logs the stages of the hits its unit takes.</summary>
    public class TargetComp : UnitComponent, IDamageTarget
    {
        /// <summary>Logs the stage.</summary>
        public void OnTaking(Hit hit) => TestLog.Lines.Add("target comp taking");

        /// <summary>Logs the stage.</summary>
        public void OnTaken(Hit hit) => TestLog.Lines.Add("target comp taken");
    }

    /// <summary>A game component that logs every hit's stages as "game".</summary>
    public class GameHitWatch : GameComponent, IDamageWatch
    {
        /// <summary>Logs the stage.</summary>
        public void OnBeforeArmor(Hit hit) => TestLog.Lines.Add("game before");

        /// <summary>Logs the stage.</summary>
        public void OnAfterArmor(Hit hit) => TestLog.Lines.Add("game after");
    }

    /// <summary>A player component that logs the stages of its player's hits, with its name.</summary>
    public class PlayerHitWatch : PlayerComponent, IDamageWatch
    {
        /// <summary>The name the log lines start with.</summary>
        private readonly string name;

        /// <summary>Makes a watch that logs under a name.</summary>
        public PlayerHitWatch(string name)
        {
            this.name = name;
        }

        /// <summary>Logs the stage.</summary>
        public void OnBeforeArmor(Hit hit) => TestLog.Lines.Add(name + " before");

        /// <summary>Logs the stage.</summary>
        public void OnAfterArmor(Hit hit) => TestLog.Lines.Add(name + " after");
    }

    /// <summary>A target component that removes its unit before armor.</summary>
    public class RemoverBeforeArmor : UnitComponent, IDamageTarget
    {
        /// <summary>Removes the unit.</summary>
        public void OnTaking(Hit hit) => Unit.Remove();
    }
}
