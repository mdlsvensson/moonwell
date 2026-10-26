namespace Moonwell.Testing
{
    /// <summary>A hero with fields of its own named as the core's fields of Actor, Unit and Hero. In Lua a class's fields share one table with its base's, so each of these must stay apart from the base's field of the same name.</summary>
    public sealed class ShadowHero : Hero
    {
        // A map cannot see the core's internal fields, so these hide nothing there; the tests can see them, and C# warns.
        // Each starts at a value the core's field of the name does not have at the end of the shadowed-fields case.
#pragma warning disable CS0108

        private string components = "own";
        private int dispatching = 7;
        private bool holes = true;
        private string abilities = "own";
        private float lastLife = -7.0f;
        private float lastMaxLife = -7.0f;
        private float lastMana = -7.0f;
        private float lastMaxMana = -7.0f;
        private float lastFacing = -7.0f;
        private int lastLevel = -7;
        private bool lastPaused = false;
        private float lastScale = -7.0f;
        private bool scaleKnown = false;
        private float lastFlyHeight = -7.0f;
        private int lastInventorySize = -7;
        private string lastName = "own";
        private bool flyReady = false;
        private int lastExperience = -7;
        private int lastStrength = -7;
        private int lastAgility = -7;
        private int lastIntelligence = -7;
        private int lastStrengthWithBonus = -7;
        private int lastAgilityWithBonus = -7;
        private int lastIntelligenceWithBonus = -7;
        private int lastSkillPoints = -7;
        private string lastProperName = "own";

#pragma warning restore CS0108

        /// <summary>The names of its own fields that are no longer at their start, each followed by a space; empty when none changed.</summary>
        public string Changed()
        {
            var changed = "";
            if (components != "own") changed += "components ";
            if (dispatching != 7) changed += "dispatching ";
            if (!holes) changed += "holes ";
            if (abilities != "own") changed += "abilities ";
            if (lastLife != -7.0f) changed += "lastLife ";
            if (lastMaxLife != -7.0f) changed += "lastMaxLife ";
            if (lastMana != -7.0f) changed += "lastMana ";
            if (lastMaxMana != -7.0f) changed += "lastMaxMana ";
            if (lastFacing != -7.0f) changed += "lastFacing ";
            if (lastLevel != -7) changed += "lastLevel ";
            if (lastPaused) changed += "lastPaused ";
            if (lastScale != -7.0f) changed += "lastScale ";
            if (scaleKnown) changed += "scaleKnown ";
            if (lastFlyHeight != -7.0f) changed += "lastFlyHeight ";
            if (lastInventorySize != -7) changed += "lastInventorySize ";
            if (lastName != "own") changed += "lastName ";
            if (flyReady) changed += "flyReady ";
            if (lastExperience != -7) changed += "lastExperience ";
            if (lastStrength != -7) changed += "lastStrength ";
            if (lastAgility != -7) changed += "lastAgility ";
            if (lastIntelligence != -7) changed += "lastIntelligence ";
            if (lastStrengthWithBonus != -7) changed += "lastStrengthWithBonus ";
            if (lastAgilityWithBonus != -7) changed += "lastAgilityWithBonus ";
            if (lastIntelligenceWithBonus != -7) changed += "lastIntelligenceWithBonus ";
            if (lastSkillPoints != -7) changed += "lastSkillPoints ";
            if (lastProperName != "own") changed += "lastProperName ";
            return changed;
        }
    }
}
