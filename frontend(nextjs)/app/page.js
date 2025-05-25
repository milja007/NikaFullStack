"use client";

import { useState, useMemo } from "react";
import { jsPDF } from "jspdf";
import styles from "./page.module.css";
import { useQuery } from "@tanstack/react-query";

const fetchQuizData = async () => {
  const response = await fetch("http://localhost:8080/api/quiz-data");
  if (!response.ok) {
    throw new Error(`HTTP error! status: ${response.status}`);
  }
  return response.json();
};

export default function Home() {
  const {
    data: quizApiData,
    isLoading,
    isError,
    error,
  } = useQuery({
    queryKey: ["quizDataFromGo"],
    queryFn: fetchQuizData,
  });

  const [selected, setSelected] = useState({});
  const [calculatedScore, setCalculatedScore] = useState(null);
  const [displayedDenominator, setDisplayedDenominator] = useState(null);

  // Memoizacija za Kategories i quizItems da se riješi ESLint upozorenje
  const Kategories = useMemo(
    () => quizApiData?.Kategories || [],
    [quizApiData]
  );
  const quizItems = useMemo(() => quizApiData?.quizItems || [], [quizApiData]);

  const totalPossibleNonRemovable = useMemo(
    () =>
      quizItems
        .filter((item) => !item.removable)
        .reduce((sum, item) => sum + (item.score > 0 ? item.score : 0), 0),
    [quizItems]
  );

  // Prilagođeno da koristi index umjesto item.id
  const handleToggle = (itemIndex) => {
    setSelected((prev) => {
      const newState = { ...prev, [itemIndex]: !prev[itemIndex] };
      return newState;
    });
    setCalculatedScore(null);
    setDisplayedDenominator(null);
  };

  // Prilagođeno da koristi index
  const calculateScore = () => {
    if (quizItems.length === 0) return;
    let currentScore = 0;
    let currentDenominator = totalPossibleNonRemovable;

    quizItems.forEach((item, index) => {
      // Koristimo index ovdje
      const itemScore = item.score > 0 ? item.score : 0;
      if (selected[index]) {
        // Provjeravamo selected[index]
        currentScore += itemScore;
        if (item.removable) {
          currentDenominator += itemScore;
        }
      }
    });
    setCalculatedScore(currentScore);
    setDisplayedDenominator(currentDenominator);
  };

  // Prilagođeno da koristi index
  const generatePDF = () => {
    if (quizItems.length === 0) return;
    const doc = new jsPDF();
    doc.setFont("helvetica");
    doc.setFontSize(16);
    doc.text("Checklist", 20, 20);
    doc.setFontSize(12);
    doc.text(`Date: ${new Date().toLocaleString()}`, 20, 30);
    if (calculatedScore !== null && displayedDenominator !== null) {
      doc.text(
        `Total Score: ${calculatedScore} / ${displayedDenominator}`,
        20,
        40
      );
    }
    let yOffset = 50;
    const maxWidth = 180;
    const lineHeight = 7;
    const itemSpacing = 3;

    quizItems.forEach((item, index) => {
      // Koristimo index ovdje
      if (selected[index]) {
        // Provjeravamo selected[index]
        let text = `${index + 1}. ${item.text} - ${
          item.score > 0 ? item.score + " pts" : "No points"
        }`;
        const textLines = doc.splitTextToSize(text, maxWidth);
        const requiredHeight = textLines.length * lineHeight + itemSpacing;
        if (yOffset + requiredHeight > 280) {
          doc.addPage();
          yOffset = 20;
        }
        textLines.forEach((line) => {
          doc.text(line, 20, yOffset);
          yOffset += lineHeight;
        });
        yOffset += itemSpacing;
      }
    });
    doc.save("Checklist.pdf");
  };

  if (isLoading) {
    return (
      <div className={styles.App}>
        <p>Učitavanje podataka...</p>
      </div>
    );
  }

  if (isError) {
    return (
      <div className={styles.App}>
        <p>Greška pri dohvaćanju podataka: {error.message}</p>
      </div>
    );
  }

  if (quizItems.length === 0 && !isLoading) {
    // Provjeri i !isLoading da se ne prikaže prerano
    return (
      <div className={styles.App}>
        <p>Nema dostupnih pitanja.</p>
      </div>
    );
  }

  return (
    <div className={styles.App}>
      <h1>Event Sustainability Checklist</h1>
      {/* Uvodni tekstovi ostaju isti */}
      <h3>How to Use This Checklist</h3>
      <br />
      <p>
        Each item on the checklist contributes to your sustainability score.
        There are two types of items:
      </p>
      <ul>
        <li>
          Mandatory items are always included in your final score. They are
          selected to align with key EU sustainability goals.
        </li>
        <li>
          Value-Added items can increase your final score if completed, but are
          not required. If they are not fulfilled, they are simply not counted.
        </li>
      </ul>
      <p>
        Some actions have multiple levels. You only select and score the level
        you have actually achieved.
      </p>
      <p>
        To successfully complete the checklist, you must reach a minimum score
        based on Mandatory items. Value-Added items can raise your score further
        but do not impact the minimum requirement.
      </p>
      <div className={styles.quizContainer}>
        {quizItems.map((item, index) => {
          // Koristimo index za key i logiku
          let categoryTitle = null;
          if (
            index === 0 ||
            (quizItems[index - 1] &&
              item.kategorija !== quizItems[index - 1].kategorija)
          ) {
            categoryTitle = <h3>{item.kategorija}</h3>;
          }

          return (
            <div key={index} id={index}>
              {" "}
              {/* Koristimo index kao key */}
              {categoryTitle}
              <label className={styles.quizItem}>
                <span className={styles.itemText}>
                  {index + 1}. {item.text}
                  {item.removable && (
                    <span className={styles.removableTag}> (Value-added)</span>
                  )}
                </span>
                <span className={styles.itemScore}>
                  {item.score > 0 ? `${item.score} pts` : "No points"}
                </span>
                <input
                  type="checkbox"
                  checked={!!selected[index]} // Koristimo selected[index]
                  onChange={() => handleToggle(index)} // Prosljeđujemo index
                />
              </label>
            </div>
          );
        })}
      </div>
      <button className={styles.submitBtn} onClick={calculateScore}>
        Calculate Score
      </button>
      {calculatedScore !== null && displayedDenominator !== null && (
        <div className={styles.result}>
          Your score: {calculatedScore} / {displayedDenominator}
        </div>
      )}
      {calculatedScore !== null && displayedDenominator !== null && (
        <button className={styles.submitBtn} onClick={generatePDF}>
          Download Results (PDF)
        </button>
      )}
    </div>
  );
}
